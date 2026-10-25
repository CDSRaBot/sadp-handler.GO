package ui_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/ui"
)

type fakeDiscovery struct {
	called bool
	result []ui.DiscoveredDevice
	err    error
}

func (f *fakeDiscovery) Discover(_ context.Context, _ *int, _ ui.DiscoveryPolicy) ([]ui.DiscoveredDevice, error) {
	f.called = true
	return f.result, f.err
}

type fakeLocales struct {
	listed    bool
	switched  string
	exported  string
	list      []string
	listError error
}

func (f *fakeLocales) ListLocales(context.Context) ([]string, error) {
	f.listed = true
	return f.list, f.listError
}

func (f *fakeLocales) SwitchLocale(_ context.Context, code string) error {
	f.switched = code
	return nil
}

func (f *fakeLocales) ExportLocaleTemplate(_ context.Context, destination string) error {
	f.exported = destination
	return nil
}

func TestDispatchCommandRoutesSafeCommands(t *testing.T) {
	locales := &fakeLocales{list: []string{"en", "ru"}}
	result, err := ui.DispatchCommand(context.Background(), ui.LanguageCommand{Action: ui.LanguageList}, ui.Dependencies{Locales: locales})
	if err != nil || !result.OK || len(result.Locales) != 2 || !locales.listed {
		t.Fatalf("language list = %#v, %v", result, err)
	}

	result, err = ui.DispatchCommand(context.Background(), ui.LanguageCommand{Action: ui.LanguageSwitch, Code: "ru"}, ui.Dependencies{Locales: locales})
	if err != nil || !result.OK || locales.switched != "ru" {
		t.Fatalf("language switch = %#v, %v", result, err)
	}

	result, err = ui.DispatchCommand(context.Background(), ui.QuitCommand{}, ui.Dependencies{})
	if err != nil || !result.OK || !result.Exit {
		t.Fatalf("quit = %#v, %v", result, err)
	}
}

func TestDispatchCommandValidatesDiscoveryBeforeCallingService(t *testing.T) {
	discovery := &fakeDiscovery{}
	result, err := ui.DispatchCommand(context.Background(), ui.DiscoverCommand{
		Policy: ui.DiscoveryPolicy{CycleTimeout: time.Second, RepeatCount: 1, MaxSADPDatagramSize: 1500},
	}, ui.Dependencies{Discovery: discovery})
	if err != nil || !result.OK || !discovery.called {
		t.Fatalf("discover = %#v, %v", result, err)
	}

	discovery.called = false
	result, err = ui.DispatchCommand(context.Background(), ui.DiscoverCommand{
		Policy: ui.DiscoveryPolicy{CycleTimeout: time.Second, RepeatCount: 101, MaxSADPDatagramSize: 1500},
	}, ui.Dependencies{Discovery: discovery})
	if err == nil || result.OK || result.Error == nil || result.Error.Code != ui.FailureInvalidArgument || discovery.called {
		t.Fatalf("invalid discover = %#v, %v", result, err)
	}
}

func TestDispatchCommandKeepsMutationGateBeforeSideEffects(t *testing.T) {
	result, err := ui.DispatchCommand(context.Background(), ui.ActivateCommand{
		Targets:       ui.TargetSelector{MACs: []net.HardwareAddr{{0, 1, 2, 3, 4, 5}}},
		PasswordInput: ui.PasswordFromStdin,
	}, ui.Dependencies{})
	if err == nil || result.OK || result.Error == nil || result.Error.Code != ui.FailureUnsupported {
		t.Fatalf("activate = %#v, %v", result, err)
	}
}

func TestDispatchCommandCanceledContextReturnsTypedFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := ui.DispatchCommand(ctx, ui.HelpCommand{}, ui.Dependencies{})
	if !errors.Is(err, context.Canceled) || result.OK || result.Error == nil || result.Error.Code != ui.FailureCanceled {
		t.Fatalf("canceled dispatch = %#v, %v", result, err)
	}
}
