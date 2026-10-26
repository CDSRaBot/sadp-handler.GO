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

type WorkflowActivationService struct {
	Workflow  logic.MutationWorkflow
	Targets   MutationTargetResolver
	Passwords PasswordProvider
}

func (s WorkflowActivationService) Activate(ctx context.Context, command ActivateCommand) ([]DeviceOutcome, error) {
	if s.Targets == nil || s.Passwords == nil {
		return nil, errors.New("activation service dependencies are incomplete")
	}
	password, err := s.Passwords.Password(ctx)
	if err != nil {
		return nil, err
	}
	return s.run(ctx, command.Targets.MACs, func(target logic.MutationTarget) (logic.MutationDecision, error) {
		return s.Workflow.Activate(ctx, target, password, command.Email)
	})
}

type WorkflowNetworkService struct {
	Workflow  logic.MutationWorkflow
	Targets   MutationTargetResolver
	Passwords PasswordProvider
}

func (s WorkflowNetworkService) ConfigureNetwork(ctx context.Context, command NetworkCommand) ([]DeviceOutcome, error) {
	if s.Targets == nil || s.Passwords == nil {
		return nil, errors.New("network service dependencies are incomplete")
	}
	password, err := s.Passwords.Password(ctx)
	if err != nil {
		return nil, err
	}
	return s.run(ctx, command.Targets.MACs, func(target logic.MutationTarget) (logic.MutationDecision, error) {
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
			outcomes = append(outcomes, DeviceOutcome{MAC: key, Status: "failed", Error: "device operation failed"})
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
