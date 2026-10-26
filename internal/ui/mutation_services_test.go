package ui_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
	"github.com/CDSRaBot/sadp-handler.GO/internal/ui"
)

type serviceTargetResolver struct {
	targets map[string]logic.MutationTarget
}

func (r serviceTargetResolver) Resolve(_ context.Context, mac net.HardwareAddr) (logic.MutationTarget, error) {
	target, ok := r.targets[mac.String()]
	if !ok {
		return logic.MutationTarget{}, errors.New("synthetic target resolution failure")
	}
	return target, nil
}

type servicePasswordProvider struct {
	password string
}

func (p servicePasswordProvider) Password(context.Context) (string, error) {
	return p.password, nil
}

func serviceTarget(mac net.HardwareAddr) logic.MutationTarget {
	return logic.MutationTarget{
		MAC:    mac,
		Sender: &net.UDPAddr{IP: net.ParseIP("192.0.2.10"), Port: 37020},
	}
}

func TestWorkflowActivationServiceMapsAcceptedAndRejectedOutcomes(t *testing.T) {
	first := net.HardwareAddr{0, 17, 34, 51, 68, 85}
	second := net.HardwareAddr{170, 187, 204, 221, 238, 255}
	service := ui.WorkflowActivationService{
		Targets: serviceTargetResolver{targets: map[string]logic.MutationTarget{
			first.String():  serviceTarget(first),
			second.String(): serviceTarget(second),
		}},
		Passwords: servicePasswordProvider{password: "ValidPass1!"},
		Workflow: logic.MutationWorkflow{
			Transport: serviceTransport{},
			Parser: serviceParser{parse: func(operation logic.MutationOperation, target logic.MutationTarget, _ net.Addr, _ []byte) (logic.MutationDecision, error) {
				status := logic.MutationAccepted
				if target.MAC.String() == second.String() {
					status = logic.MutationRejected
				}
				return logic.MutationDecision{Operation: operation, Target: target, Status: status}, nil
			}},
		},
	}

	outcomes, err := service.Activate(context.Background(), ui.ActivateCommand{
		Targets:       ui.TargetSelector{MACs: []net.HardwareAddr{first, second}, UseTUISelection: true},
		PasswordInput: ui.PasswordFromStdin,
		Confirm:       true,
	})
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if len(outcomes) != 2 || outcomes[0].Status != "accepted" || outcomes[1].Status != "rejected" {
		t.Fatalf("outcomes = %#v", outcomes)
	}
}

func TestWorkflowNetworkServicePreservesPartialResolutionFailure(t *testing.T) {
	first := net.HardwareAddr{0, 17, 34, 51, 68, 85}
	second := net.HardwareAddr{170, 187, 204, 221, 238, 255}
	service := ui.WorkflowNetworkService{
		Targets:   serviceTargetResolver{targets: map[string]logic.MutationTarget{first.String(): serviceTarget(first)}},
		Passwords: servicePasswordProvider{password: "ValidPass1!"},
		Workflow: logic.MutationWorkflow{
			ChallengeProvider: serviceChallengeProvider{target: serviceTarget(first)},
			Transport:         serviceTransport{},
			Parser: serviceParser{parse: func(operation logic.MutationOperation, target logic.MutationTarget, _ net.Addr, _ []byte) (logic.MutationDecision, error) {
				return logic.MutationDecision{Operation: operation, Target: target, Status: logic.MutationAccepted}, nil
			}},
		},
	}

	outcomes, err := service.ConfigureNetwork(context.Background(), ui.NetworkCommand{
		Targets:       ui.TargetSelector{MACs: []net.HardwareAddr{first, second}},
		PasswordInput: ui.PasswordFromStdin,
		Config:        logic.NetworkConfig{UseDHCP: true},
		Confirm:       true,
	})
	if err == nil || len(outcomes) != 2 {
		t.Fatalf("outcomes = %#v, error = %v", outcomes, err)
	}
	if outcomes[0].Status != "accepted" || outcomes[1].Status != "failed" {
		t.Fatalf("outcomes = %#v", outcomes)
	}
}

type serviceTransport struct{}

func (serviceTransport) Exchange(_ context.Context, target logic.MutationTarget, _ []byte) (logic.MutationExchange, error) {
	return logic.MutationExchange{Sender: target.Sender, Data: []byte("synthetic-response")}, nil
}

type serviceParser struct {
	parse func(logic.MutationOperation, logic.MutationTarget, net.Addr, []byte) (logic.MutationDecision, error)
}

func (p serviceParser) Parse(operation logic.MutationOperation, target logic.MutationTarget, sender net.Addr, data []byte) (logic.MutationDecision, error) {
	return p.parse(operation, target, sender, data)
}

type serviceChallengeProvider struct {
	target logic.MutationTarget
}

func (p serviceChallengeProvider) Acquire(context.Context, logic.MutationTarget) (logic.Challenge, error) {
	return logic.Challenge{
		Value:     "synthetic-challenge",
		TargetMAC: p.target.MAC,
		Sender:    p.target.Sender,
		ExpiresAt: time.Now().Add(time.Minute),
	}, nil
}
