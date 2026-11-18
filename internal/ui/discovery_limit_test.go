package ui

import (
	"bytes"
	"context"
	"testing"
)

type capturedDiscoveryPolicy struct {
	policy DiscoveryPolicy
}

func (s *capturedDiscoveryPolicy) Discover(_ context.Context, _ *int, policy DiscoveryPolicy) ([]DiscoveredDevice, error) {
	s.policy = policy
	return nil, nil
}

func TestOneShotDiscoveryUsesConfiguredDatagramLimit(t *testing.T) {
	service := &capturedDiscoveryPolicy{}
	dependencies := Dependencies{
		Discovery: service, DiscoveryPolicy: DiscoveryPolicy{MaxSADPDatagramSize: 65507},
	}
	var output, diagnostics bytes.Buffer
	exit := runOneShot(context.Background(), []string{"discover", "--interface", "8", "--timeout", "1s"},
		&output, &diagnostics, dependencies)
	if exit != ExitSuccess || service.policy.MaxSADPDatagramSize != 65507 {
		t.Fatalf("exit=%d policy=%+v output=%q diagnostics=%q", exit, service.policy, output.String(), diagnostics.String())
	}
}
