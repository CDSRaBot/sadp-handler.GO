package logic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

type MutationOperation string

const (
	MutationActivation MutationOperation = "activation"
	MutationNetwork    MutationOperation = "network"
)

type MutationTarget struct {
	MAC    net.HardwareAddr
	Sender net.Addr
}

type MutationExchange struct {
	Sender net.Addr
	Data   []byte
}

type MutationTransport interface {
	Exchange(context.Context, MutationTarget, []byte) (MutationExchange, error)
}

type Challenge struct {
	Value     string
	TargetMAC net.HardwareAddr
	Sender    net.Addr
	ExpiresAt time.Time
}

type ChallengeProvider interface {
	Acquire(context.Context, MutationTarget) (Challenge, error)
}

type MutationDecisionStatus string

const (
	MutationAccepted MutationDecisionStatus = "accepted"
	MutationRejected MutationDecisionStatus = "rejected"
)

type MutationDecision struct {
	Operation MutationOperation
	Target    MutationTarget
	Status    MutationDecisionStatus
	Reason    string
}

type ResponseParser interface {
	Parse(MutationOperation, MutationTarget, net.Addr, []byte) (MutationDecision, error)
}

type MutationOutcome struct {
	Target    MutationTarget
	Status    MutationDecisionStatus
	Reason    string
	ErrorKind DeviceOperationErrorKind
}

type MutationWorkflow struct {
	Transport         MutationTransport
	ChallengeProvider ChallengeProvider
	Parser            ResponseParser
	MaxResponseSize   int
}

func (w MutationWorkflow) Activate(ctx context.Context, target MutationTarget, password, email string) (MutationDecision, error) {
	if err := validateMutationContextTarget(ctx, target); err != nil {
		return MutationDecision{}, err
	}
	if w.Transport == nil || w.Parser == nil {
		return MutationDecision{}, operationError(OperationInvalidInput, errors.New("mutation workflow dependencies are incomplete"))
	}
	payload, err := BuildActivationXML(ActivationRequest{
		MAC:      target.MAC,
		Password: password,
		Email:    email,
	})
	if err != nil {
		return MutationDecision{}, operationError(OperationInvalidInput, err)
	}
	return w.exchangeAndParse(ctx, MutationActivation, target, payload)
}

func (w MutationWorkflow) ConfigureNetwork(ctx context.Context, target MutationTarget, config NetworkConfig, password string) (MutationDecision, error) {
	if err := validateMutationContextTarget(ctx, target); err != nil {
		return MutationDecision{}, err
	}
	if w.ChallengeProvider == nil {
		return MutationDecision{}, operationError(OperationMissingData, ErrEmptyChallenge)
	}
	challenge, err := w.ChallengeProvider.Acquire(ctx, target)
	if err != nil {
		var operationErr *DeviceOperationError
		if errors.As(err, &operationErr) && operationErr.Kind == OperationMissingData {
			return MutationDecision{}, operationError(OperationMissingData, ErrEmptyChallenge)
		}
		return MutationDecision{}, normalizeMutationError(err, OperationTransport)
	}
	if err := validateChallenge(target, challenge); err != nil {
		return MutationDecision{}, err
	}
	payload, err := BuildNetworkXML(target.MAC, config, challenge.Value, password)
	if err != nil {
		return MutationDecision{}, operationError(OperationInvalidInput, err)
	}
	return w.exchangeAndParse(ctx, MutationNetwork, target, payload)
}

func (w MutationWorkflow) ActivateMany(ctx context.Context, targets []MutationTarget, password, email string) ([]MutationOutcome, error) {
	return w.runMany(ctx, targets, func(target MutationTarget) (MutationDecision, error) {
		return w.Activate(ctx, target, password, email)
	})
}

func (w MutationWorkflow) ConfigureNetworkMany(ctx context.Context, targets []MutationTarget, config NetworkConfig, password string) ([]MutationOutcome, error) {
	return w.runMany(ctx, targets, func(target MutationTarget) (MutationDecision, error) {
		return w.ConfigureNetwork(ctx, target, config, password)
	})
}

func (w MutationWorkflow) exchangeAndParse(ctx context.Context, operation MutationOperation, target MutationTarget, payload []byte) (MutationDecision, error) {
	exchange, err := w.Transport.Exchange(ctx, target, payload)
	if err != nil {
		return MutationDecision{}, normalizeMutationError(err, OperationTransport)
	}
	if exchange.Sender == nil || len(exchange.Data) == 0 {
		return MutationDecision{}, operationError(OperationMalformed, errors.New("mutation response is empty"))
	}
	if w.MaxResponseSize > 0 && len(exchange.Data) > w.MaxResponseSize {
		return MutationDecision{}, operationError(OperationMalformed, errors.New("mutation response exceeds configured size"))
	}
	decision, err := w.Parser.Parse(operation, target, exchange.Sender, exchange.Data)
	if err != nil {
		return MutationDecision{}, normalizeMutationError(err, OperationMalformed)
	}
	if decision.Operation != operation || decision.Status != MutationAccepted && decision.Status != MutationRejected {
		return MutationDecision{}, operationError(OperationMalformed, errors.New("mutation response decision is invalid"))
	}
	if !sameMAC(decision.Target.MAC, target.MAC) {
		return MutationDecision{}, operationError(OperationCorrelation, errors.New("mutation response target mismatch"))
	}
	decision.Target = target
	return decision, nil
}

func (w MutationWorkflow) runMany(ctx context.Context, targets []MutationTarget, run func(MutationTarget) (MutationDecision, error)) ([]MutationOutcome, error) {
	if len(targets) == 0 {
		return nil, operationError(OperationInvalidInput, errors.New("mutation targets are empty"))
	}
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if err := validateMutationContextTarget(ctx, target); err != nil {
			return nil, err
		}
		key := target.MAC.String() + "|" + target.Sender.String()
		if _, exists := seen[key]; exists {
			return nil, operationError(OperationInvalidInput, errors.New("mutation targets contain duplicates"))
		}
		seen[key] = struct{}{}
	}
	outcomes := make([]MutationOutcome, 0, len(targets))
	var aggregate error
	for _, target := range targets {
		decision, err := run(target)
		if err != nil {
			kind := operationKind(err)
			outcomes = append(outcomes, MutationOutcome{Target: target, ErrorKind: kind, Reason: safeFailureReason(kind)})
			aggregate = errors.Join(aggregate, err)
			continue
		}
		outcomes = append(outcomes, MutationOutcome{Target: target, Status: decision.Status, Reason: decision.Reason})
	}
	return outcomes, aggregate
}

func validateMutationContextTarget(ctx context.Context, target MutationTarget) error {
	if ctx == nil {
		return operationError(OperationInvalidInput, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return operationError(OperationCanceled, err)
	}
	if len(target.MAC) != 6 || target.Sender == nil {
		return operationError(OperationInvalidInput, errors.New("mutation target is invalid"))
	}
	return nil
}

func validateChallenge(target MutationTarget, challenge Challenge) error {
	if challenge.Value == "" {
		return operationError(OperationMissingData, ErrEmptyChallenge)
	}
	if !sameMAC(challenge.TargetMAC, target.MAC) || !sameAddress(challenge.Sender, target.Sender) {
		return operationError(OperationCorrelation, errors.New("challenge target mismatch"))
	}
	if !challenge.ExpiresAt.After(time.Now()) {
		return operationError(OperationMissingData, errors.New("challenge is expired"))
	}
	return nil
}

func normalizeMutationError(err error, fallback DeviceOperationErrorKind) error {
	var operationErr *DeviceOperationError
	if errors.As(err, &operationErr) {
		if operationErr.Kind == OperationMalformed || operationErr.Kind == OperationCorrelation {
			return operationError(operationErr.Kind, redactedCause{
				err:     err,
				message: "response validation failed",
			})
		}
		return operationErr
	}
	if errors.Is(err, context.Canceled) {
		return operationError(OperationCanceled, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return operationError(OperationTimeout, err)
	}
	if fallback == OperationMalformed || fallback == OperationCorrelation {
		return operationError(fallback, redactedCause{
			err:     err,
			message: "response validation failed",
		})
	}
	return operationError(fallback, err)
}

func operationError(kind DeviceOperationErrorKind, err error) error {
	return &DeviceOperationError{Kind: kind, Err: err}
}

func operationKind(err error) DeviceOperationErrorKind {
	var operationErr *DeviceOperationError
	if errors.As(err, &operationErr) {
		return operationErr.Kind
	}
	return OperationTransport
}

func safeFailureReason(kind DeviceOperationErrorKind) string {
	if kind == "" {
		return "operation_failed"
	}
	return fmt.Sprintf("operation failed (%s)", kind)
}

func sameMAC(left, right net.HardwareAddr) bool {
	return len(left) == len(right) && len(left) == 6 && left.String() == right.String()
}

func sameAddress(left, right net.Addr) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.String() == right.String()
}
