package logic_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

type fakeTransport struct {
	exchange func(context.Context, logic.MutationTarget, []byte) (logic.MutationExchange, error)
}

func (f fakeTransport) Exchange(ctx context.Context, target logic.MutationTarget, payload []byte) (logic.MutationExchange, error) {
	return f.exchange(ctx, target, payload)
}

type fakeChallengeProvider struct {
	acquire func(context.Context, logic.MutationTarget) (logic.Challenge, error)
}

func (f fakeChallengeProvider) Acquire(ctx context.Context, target logic.MutationTarget) (logic.Challenge, error) {
	return f.acquire(ctx, target)
}

type fakeResponseParser struct {
	parse func(logic.MutationOperation, logic.MutationTarget, net.Addr, []byte) (logic.MutationDecision, error)
}

func (f fakeResponseParser) Parse(operation logic.MutationOperation, target logic.MutationTarget, sender net.Addr, data []byte) (logic.MutationDecision, error) {
	return f.parse(operation, target, sender, data)
}

func mutationTarget() logic.MutationTarget {
	return logic.MutationTarget{
		MAC:    net.HardwareAddr{0, 17, 34, 51, 68, 85},
		Sender: &net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 37020},
	}
}

func validPassword() string {
	return "ValidPass1!"
}

func networkConfig() logic.NetworkConfig {
	return logic.NetworkConfig{UseDHCP: true}
}

func TestActivationWorkflowReturnsAcceptedDecision(t *testing.T) {
	target := mutationTarget()
	workflow := logic.MutationWorkflow{
		Transport: fakeTransport{exchange: func(ctx context.Context, got logic.MutationTarget, payload []byte) (logic.MutationExchange, error) {
			if len(payload) == 0 {
				t.Fatal("workflow sent an empty activation payload")
			}
			return logic.MutationExchange{Sender: got.Sender, Data: []byte("accepted")}, nil
		}},
		Parser: fakeResponseParser{parse: func(operation logic.MutationOperation, got logic.MutationTarget, sender net.Addr, data []byte) (logic.MutationDecision, error) {
			return logic.MutationDecision{Operation: operation, Target: got, Status: logic.MutationAccepted}, nil
		}},
	}

	decision, err := workflow.Activate(context.Background(), target, validPassword(), "")
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if decision.Status != logic.MutationAccepted {
		t.Fatalf("decision status = %q, want accepted", decision.Status)
	}
}

func TestNetworkWorkflowFailsClosedWithoutChallenge(t *testing.T) {
	workflow := logic.MutationWorkflow{
		ChallengeProvider: fakeChallengeProvider{acquire: func(context.Context, logic.MutationTarget) (logic.Challenge, error) {
			return logic.Challenge{}, &logic.DeviceOperationError{Kind: logic.OperationMissingData}
		}},
	}

	_, err := workflow.ConfigureNetwork(context.Background(), mutationTarget(), networkConfig(), validPassword())
	if err == nil || !errors.Is(err, logic.ErrEmptyChallenge) {
		t.Fatalf("ConfigureNetwork() error = %v, want missing challenge", err)
	}
}

func TestMutationWorkflowRedactsSecretsFromErrors(t *testing.T) {
	password := validPassword()
	challenge := "synthetic-challenge"
	workflow := logic.MutationWorkflow{
		ChallengeProvider: fakeChallengeProvider{acquire: func(context.Context, logic.MutationTarget) (logic.Challenge, error) {
			return logic.Challenge{Value: challenge, TargetMAC: mutationTarget().MAC, Sender: mutationTarget().Sender, ExpiresAt: time.Now().Add(time.Minute)}, nil
		}},
		Transport: fakeTransport{exchange: func(context.Context, logic.MutationTarget, []byte) (logic.MutationExchange, error) {
			return logic.MutationExchange{}, errors.New("transport failed")
		}},
	}

	_, err := workflow.ConfigureNetwork(context.Background(), mutationTarget(), networkConfig(), password)
	if err == nil || strings.Contains(err.Error(), password) || strings.Contains(err.Error(), challenge) {
		t.Fatalf("ConfigureNetwork() leaked secret: %v", err)
	}
}

func TestMutationWorkflowPreservesPartialOutcomes(t *testing.T) {
	targets := []logic.MutationTarget{mutationTarget(), {
		MAC:    net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		Sender: &net.UDPAddr{IP: net.ParseIP("192.0.2.2"), Port: 37020},
	}}
	workflow := logic.MutationWorkflow{
		Transport: fakeTransport{exchange: func(ctx context.Context, target logic.MutationTarget, payload []byte) (logic.MutationExchange, error) {
			if target.MAC.String() == "aa:bb:cc:dd:ee:ff" {
				return logic.MutationExchange{}, &logic.DeviceOperationError{Kind: logic.OperationRejected}
			}
			return logic.MutationExchange{Sender: target.Sender, Data: []byte("accepted")}, nil
		}},
		Parser: fakeResponseParser{parse: func(operation logic.MutationOperation, target logic.MutationTarget, sender net.Addr, data []byte) (logic.MutationDecision, error) {
			return logic.MutationDecision{Operation: operation, Target: target, Status: logic.MutationAccepted}, nil
		}},
	}

	outcomes, err := workflow.ActivateMany(context.Background(), targets, validPassword(), "")
	if err == nil || len(outcomes) != len(targets) {
		t.Fatalf("ActivateMany() outcomes=%d err=%v, want two outcomes and aggregate error", len(outcomes), err)
	}
	if outcomes[0].Status != logic.MutationAccepted || outcomes[1].Status == logic.MutationAccepted {
		t.Fatalf("unexpected partial outcomes: %#v", outcomes)
	}
}

func TestMutationWorkflowRejectsDuplicateTargetsBeforeTransportUse(t *testing.T) {
	target := mutationTarget()
	called := false
	workflow := logic.MutationWorkflow{
		Transport: fakeTransport{exchange: func(context.Context, logic.MutationTarget, []byte) (logic.MutationExchange, error) {
			called = true
			return logic.MutationExchange{}, nil
		}},
		Parser: fakeResponseParser{parse: func(logic.MutationOperation, logic.MutationTarget, net.Addr, []byte) (logic.MutationDecision, error) {
			return logic.MutationDecision{}, nil
		}},
	}

	_, err := workflow.ActivateMany(context.Background(), []logic.MutationTarget{target, target}, validPassword(), "")
	var operationErr *logic.DeviceOperationError
	if !errors.As(err, &operationErr) || operationErr.Kind != logic.OperationInvalidInput {
		t.Fatalf("ActivateMany() error = %v, want invalid-input error", err)
	}
	if called {
		t.Fatal("duplicate target validation must happen before transport use")
	}
}
