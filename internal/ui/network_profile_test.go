package ui_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
	"github.com/CDSRaBot/sadp-handler.GO/internal/ui"
)

type rejectedProfile struct{}

func (rejectedProfile) VerifyNetworkProfile(context.Context, logic.MutationTarget) error {
	return errors.New("profile is unsupported")
}

func TestNetworkServiceRejectsUnverifiedTarget(t *testing.T) {
	mac := net.HardwareAddr{0, 17, 34, 51, 68, 85}
	service := ui.WorkflowNetworkService{
		Targets:         serviceTargetResolver{targets: map[string]logic.MutationTarget{mac.String(): serviceTarget(mac)}},
		Passwords:       servicePasswordProvider{password: "SyntheticPassword1!"},
		ProfileVerifier: rejectedProfile{},
	}
	outcomes, err := service.ConfigureNetwork(context.Background(), ui.NetworkCommand{
		Targets: ui.TargetSelector{MACs: []net.HardwareAddr{mac}},
	})
	if !errors.Is(err, ui.ErrSADPNetworkProfileUnverified) || len(outcomes) != 1 ||
		outcomes[0].Error != ui.ErrSADPNetworkProfileUnverified.Error() {
		t.Fatalf("outcomes = %#v, error = %v", outcomes, err)
	}
}
