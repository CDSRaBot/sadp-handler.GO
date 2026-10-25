package ui_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
	"github.com/CDSRaBot/sadp-handler.GO/internal/ui"
)

type fakeActivation struct {
	called bool
}

func (f *fakeActivation) Activate(context.Context, ui.ActivateCommand) ([]ui.DeviceOutcome, error) {
	f.called = true
	return []ui.DeviceOutcome{{MAC: "00:11:22:33:44:55", Status: "activated"}}, nil
}

type fakeNetwork struct {
	err error
}

func (f *fakeNetwork) ConfigureNetwork(context.Context, ui.NetworkCommand) ([]ui.DeviceOutcome, error) {
	return nil, f.err
}

func TestDispatchMutationRoutesInjectedServices(t *testing.T) {
	service := &fakeActivation{}
	result, err := ui.DispatchCommand(context.Background(), ui.ActivateCommand{
		Targets:       ui.TargetSelector{MACs: []net.HardwareAddr{{0, 1, 2, 3, 4, 5}}},
		PasswordInput: ui.PasswordFromStdin,
	}, ui.Dependencies{Activation: service})
	if err != nil || !result.OK || !service.called || len(result.Devices) != 1 {
		t.Fatalf("activation = %#v, %v", result, err)
	}
}

func TestDispatchMutationMapsTypedFailures(t *testing.T) {
	service := &fakeNetwork{err: &logic.DeviceOperationError{Kind: logic.OperationInvalidInput, Err: errors.New("invalid")}}
	result, err := ui.DispatchCommand(context.Background(), ui.NetworkCommand{
		Targets:       ui.TargetSelector{MACs: []net.HardwareAddr{{0, 1, 2, 3, 4, 5}}},
		PasswordInput: ui.PasswordFromStdin,
	}, ui.Dependencies{Network: service})
	if err == nil || result.OK || result.Error == nil || result.Error.Code != ui.FailureInvalidArgument {
		t.Fatalf("network failure = %#v, %v", result, err)
	}
}

func TestDispatchMutationMapsCanceledOperation(t *testing.T) {
	service := &fakeNetwork{err: &logic.DeviceOperationError{Kind: logic.OperationCanceled, Err: context.Canceled}}
	result, err := ui.DispatchCommand(context.Background(), ui.NetworkCommand{
		Targets:       ui.TargetSelector{MACs: []net.HardwareAddr{{0, 1, 2, 3, 4, 5}}},
		PasswordInput: ui.PasswordFromStdin,
	}, ui.Dependencies{Network: service})
	if !errors.Is(err, context.Canceled) || result.OK || result.Error == nil || result.Error.Code != ui.FailureCanceled {
		t.Fatalf("canceled network = %#v, %v", result, err)
	}
}

func TestDispatchMutationRejectsUnconfirmedBulkBeforeService(t *testing.T) {
	service := &fakeActivation{}
	result, err := ui.DispatchCommand(context.Background(), ui.ActivateCommand{
		Targets: ui.TargetSelector{
			MACs: []net.HardwareAddr{{0, 1, 2, 3, 4, 5}, {6, 7, 8, 9, 10, 11}},
		},
		PasswordInput: ui.PasswordFromStdin,
	}, ui.Dependencies{Activation: service})
	if err == nil || result.Error == nil || result.Error.Code != ui.FailureInvalidArgument || service.called {
		t.Fatalf("unconfirmed bulk = %#v, %v", result, err)
	}
}
