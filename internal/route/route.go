// Package route selects SADP or ISAPI for one device operation.
package route

import (
	"context"
	"crypto/aes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
)

var ErrNotImplemented = errors.New("not implemented")

type Strategy string

const (
	StrategySADPFirst Strategy = "sadp_first"
	StrategyISAPIOnly Strategy = "isapi_only"
)

type Operation string

const (
	OperationDiscovery  Operation = "discovery"
	OperationActivation Operation = "activation"
	OperationNetwork    Operation = "network"
	OperationStatus     Operation = "status"
)

type ErrorKind string

const (
	ErrorInvalidInput    ErrorKind = "invalid_input"
	ErrorNoRoute         ErrorKind = "no_route"
	ErrorSADPUnavailable ErrorKind = "sadp_unavailable"
	ErrorUnsupported     ErrorKind = "unsupported"
	ErrorRejected        ErrorKind = "device_rejected"
	ErrorAuthentication  ErrorKind = "authentication"
	ErrorMalformed       ErrorKind = "malformed_response"
	ErrorTimeout         ErrorKind = "timeout"
	ErrorCanceled        ErrorKind = "canceled"
	ErrorVerification    ErrorKind = "verification"
)

type Error struct {
	Kind ErrorKind
	Err  error
}

func (e *Error) Error() string { return string(e.Kind) }
func (e *Error) Unwrap() error { return e.Err }

type FallbackDecision string

const (
	FallbackNotAllowed       FallbackDecision = "not_allowed"
	FallbackEligible         FallbackDecision = "eligible"
	FallbackRouteUnavailable FallbackDecision = "route_unavailable"
)

type Target struct {
	MAC       net.HardwareAddr
	Address   netip.Addr
	Interface string
}

type Result struct {
	Operation Operation
	Protocol  string
	Target    Target
	Fallback  FallbackDecision
	Err       error
}

type Transport interface {
	Execute(context.Context, Operation, Target) (Result, error)
}

type RouteProvider interface {
	CanReach(context.Context, netip.Addr, string) (bool, error)
}

type CapabilityProvider interface {
	Supports(context.Context, Target, Operation) (bool, error)
}

type Dependencies struct {
	SADP       Transport
	ISAPI      Transport
	Routes     RouteProvider
	Capability CapabilityProvider
}

type Selector struct {
	strategy Strategy
	deps     Dependencies
}

func NewSelector(strategy Strategy, dependencies Dependencies) (*Selector, error) {
	if strategy == "" {
		strategy = StrategySADPFirst
	}
	if strategy != StrategySADPFirst && strategy != StrategyISAPIOnly {
		return nil, &Error{Kind: ErrorInvalidInput, Err: fmt.Errorf("unknown strategy %q", strategy)}
	}
	if dependencies.ISAPI == nil && strategy == StrategyISAPIOnly {
		return nil, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI transport is required")}
	}
	if dependencies.SADP == nil && strategy == StrategySADPFirst {
		return nil, &Error{Kind: ErrorInvalidInput, Err: errors.New("SADP transport is required")}
	}
	return &Selector{strategy: strategy, deps: dependencies}, nil
}

func (s *Selector) Execute(ctx context.Context, operation Operation, target Target) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{Operation: operation, Target: target}, &Error{Kind: ErrorCanceled, Err: err}
	}
	if operation == "" || !target.Address.IsValid() {
		return Result{Operation: operation, Target: target}, &Error{Kind: ErrorInvalidInput, Err: errors.New("operation and target address are required")}
	}
	if s.strategy == StrategyISAPIOnly {
		return s.executeISAPI(ctx, operation, target, FallbackNotAllowed)
	}
	sadpResult, err := s.deps.SADP.Execute(ctx, operation, target)
	if err == nil {
		if sadpResult.Protocol == "" {
			sadpResult.Protocol = "sadp"
		}
		return sadpResult, nil
	}
	if !isSADPUnavailable(err) {
		return Result{Operation: operation, Target: target, Protocol: "sadp", Fallback: FallbackNotAllowed, Err: err}, err
	}
	if s.deps.Routes == nil {
		return Result{Operation: operation, Target: target, Protocol: "sadp", Fallback: FallbackRouteUnavailable, Err: err},
			&Error{Kind: ErrorNoRoute, Err: err}
	}
	reachable, routeErr := s.deps.Routes.CanReach(ctx, target.Address, target.Interface)
	if routeErr != nil {
		return Result{Operation: operation, Target: target, Protocol: "sadp", Fallback: FallbackRouteUnavailable, Err: routeErr},
			&Error{Kind: ErrorNoRoute, Err: routeErr}
	}
	if !reachable {
		return Result{Operation: operation, Target: target, Protocol: "sadp", Fallback: FallbackRouteUnavailable, Err: err},
			&Error{Kind: ErrorNoRoute, Err: err}
	}
	if s.deps.Capability == nil {
		return Result{Operation: operation, Target: target, Protocol: "sadp", Fallback: FallbackNotAllowed, Err: err},
			&Error{Kind: ErrorUnsupported, Err: errors.New("ISAPI capability provider is required")}
	}
	supported, capabilityErr := s.deps.Capability.Supports(ctx, target, operation)
	if capabilityErr != nil {
		return Result{Operation: operation, Target: target, Protocol: "sadp", Fallback: FallbackNotAllowed, Err: capabilityErr},
			&Error{Kind: ErrorUnsupported, Err: capabilityErr}
	}
	if !supported {
		return Result{Operation: operation, Target: target, Protocol: "sadp", Fallback: FallbackNotAllowed, Err: errors.New("ISAPI operation unsupported")},
			&Error{Kind: ErrorUnsupported, Err: errors.New("ISAPI operation unsupported")}
	}
	return s.executeISAPI(ctx, operation, target, FallbackEligible)
}

func (s *Selector) executeISAPI(ctx context.Context, operation Operation, target Target, fallback FallbackDecision) (Result, error) {
	result, err := s.deps.ISAPI.Execute(ctx, operation, target)
	if err != nil {
		return Result{Operation: operation, Target: target, Protocol: "isapi", Fallback: fallback, Err: err}, err
	}
	if result.Protocol == "" {
		result.Protocol = "isapi"
	}
	result.Fallback = fallback
	return result, nil
}

func isSADPUnavailable(err error) bool {
	var routeErr *Error
	return errors.As(err, &routeErr) && routeErr.Kind == ErrorSADPUnavailable
}

type ActivationSecurityMaterial struct {
	PublicKey  string
	PrivateKey *rsa.PrivateKey
	Password   string
}

func NewActivationSecurityMaterial(password string) (ActivationSecurityMaterial, error) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return ActivationSecurityMaterial{}, fmt.Errorf("generate activation key: %w", err)
	}
	modulus := strings.ToUpper(hex.EncodeToString(key.PublicKey.N.Bytes()))
	return ActivationSecurityMaterial{
		PublicKey:  base64.StdEncoding.EncodeToString([]byte(modulus)),
		PrivateKey: key,
		Password:   password,
	}, nil
}

func BuildISAPIActivationPassword(material ActivationSecurityMaterial, challengeValue string) (string, error) {
	if material.PrivateKey == nil || material.Password == "" {
		return "", &Error{Kind: ErrorInvalidInput, Err: errors.New("activation key and password are required")}
	}
	ciphertext, err := base64.StdEncoding.DecodeString(challengeValue)
	if err != nil {
		return "", &Error{Kind: ErrorMalformed, Err: fmt.Errorf("decode challenge: %w", err)}
	}
	encodedCiphertext, err := hex.DecodeString(string(ciphertext))
	if err != nil {
		return "", &Error{Kind: ErrorMalformed, Err: fmt.Errorf("decode challenge hex: %w", err)}
	}
	plain, err := rsa.DecryptPKCS1v15(rand.Reader, material.PrivateKey, encodedCiphertext)
	if err != nil {
		return "", &Error{Kind: ErrorMalformed, Err: fmt.Errorf("decrypt challenge: %w", err)}
	}
	challenge := string(plain)
	if len(challenge) != 32 {
		return "", &Error{Kind: ErrorMalformed, Err: errors.New("challenge must contain 32 hexadecimal characters")}
	}
	aesKey, err := hex.DecodeString(challenge)
	if err != nil || len(aesKey) != 16 {
		return "", &Error{Kind: ErrorMalformed, Err: errors.New("challenge is not a valid AES-128 key")}
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", &Error{Kind: ErrorMalformed, Err: err}
	}
	out := make([]byte, 0, aes.BlockSize*2)
	for _, value := range []string{challenge[:16], material.Password} {
		input := make([]byte, aes.BlockSize)
		copy(input, []byte(value))
		encrypted := make([]byte, aes.BlockSize)
		block.Encrypt(encrypted, input)
		out = append(out, []byte(strings.ToUpper(hex.EncodeToString(encrypted)))...)
	}
	return base64.StdEncoding.EncodeToString(out), nil
}
