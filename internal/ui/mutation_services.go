package ui

import (
	"context"
	"errors"
	"net"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

type MutationTargetResolver interface {
	Resolve(context.Context, net.HardwareAddr) (logic.MutationTarget, error)
}

type PasswordProvider interface {
	Password(context.Context) (string, error)
}

type SADPNetworkProfileVerifier interface {
	VerifyNetworkProfile(context.Context, logic.MutationTarget) error
}

var ErrSADPActivationDisabled = errors.New("SADP activation is disabled; ISAPI activation is not available")
var ErrSADPNetworkProfileUnverified = errors.New("SADP network profile is not verified")

type WorkflowActivationService struct {
	Workflow  logic.MutationWorkflow
	Targets   MutationTargetResolver
	Passwords PasswordProvider
}

func (WorkflowActivationService) SADPActivationDisabled() bool {
	return true
}

func (s WorkflowActivationService) Activate(ctx context.Context, command ActivateCommand) ([]DeviceOutcome, error) {
	return nil, ErrSADPActivationDisabled
}

type WorkflowNetworkService struct {
	Workflow        logic.MutationWorkflow
	Targets         MutationTargetResolver
	Passwords       PasswordProvider
	ProfileVerifier SADPNetworkProfileVerifier
}

func (s WorkflowNetworkService) ConfigureNetwork(ctx context.Context, command NetworkCommand) ([]DeviceOutcome, error) {
	if s.Targets == nil || s.Passwords == nil {
		return nil, errors.New("network service dependencies are incomplete")
	}
	if s.ProfileVerifier == nil {
		return nil, ErrSADPNetworkProfileUnverified
	}
	password, err := s.Passwords.Password(ctx)
	if err != nil {
		return nil, err
	}
	return s.run(ctx, command.Targets.MACs, func(target logic.MutationTarget) (logic.MutationDecision, error) {
		if err := s.ProfileVerifier.VerifyNetworkProfile(ctx, target); err != nil {
			return logic.MutationDecision{}, errors.Join(ErrSADPNetworkProfileUnverified, err)
		}
		return s.Workflow.ConfigureNetwork(ctx, target, command.Config, password)
	})
}

func (s WorkflowActivationService) run(ctx context.Context, macs []net.HardwareAddr, execute func(logic.MutationTarget) (logic.MutationDecision, error)) ([]DeviceOutcome, error) {
	return runWorkflowTargets(ctx, s.Targets, macs, execute)
}

func (s WorkflowNetworkService) run(ctx context.Context, macs []net.HardwareAddr, execute func(logic.MutationTarget) (logic.MutationDecision, error)) ([]DeviceOutcome, error) {
	return runWorkflowTargets(ctx, s.Targets, macs, execute)
}

func runWorkflowTargets(ctx context.Context, resolver MutationTargetResolver, macs []net.HardwareAddr, execute func(logic.MutationTarget) (logic.MutationDecision, error)) ([]DeviceOutcome, error) {
	if ctx == nil || resolver == nil || execute == nil {
		return nil, &logic.DeviceOperationError{Kind: logic.OperationInvalidInput, Err: errors.New("mutation service arguments are incomplete")}
	}
	outcomes := make([]DeviceOutcome, 0, len(macs))
	var aggregate error
	seen := make(map[string]struct{}, len(macs))
	for _, mac := range macs {
		key := mac.String()
		if _, ok := seen[key]; ok {
			return nil, &logic.DeviceOperationError{Kind: logic.OperationInvalidInput, Err: errors.New("duplicate target device")}
		}
		seen[key] = struct{}{}
		target, err := resolver.Resolve(ctx, mac)
		if err != nil {
			outcomes = append(outcomes, DeviceOutcome{MAC: key, Status: "failed", Error: "target resolution failed"})
			aggregate = errors.Join(aggregate, err)
			continue
		}
		decision, err := execute(target)
		if err != nil {
			message := "device operation failed"
			if errors.Is(err, ErrSADPNetworkProfileUnverified) {
				message = ErrSADPNetworkProfileUnverified.Error()
			}
			outcomes = append(outcomes, DeviceOutcome{MAC: key, Status: "failed", Error: message})
			aggregate = errors.Join(aggregate, err)
			continue
		}
		status := string(decision.Status)
		if status == "" {
			status = "failed"
		}
		outcome := DeviceOutcome{MAC: key, Status: status}
		if target.Sender != nil {
			outcome.Sender = target.Sender.String()
		}
		if decision.Reason != "" {
			outcome.Error = decision.Reason
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes, aggregate
}
