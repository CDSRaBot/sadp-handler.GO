package ui

import (
	"context"
	"errors"
	"sort"
)

type SessionDiscoveryService struct {
	Sessions DiscoverySessionService
}

func (s SessionDiscoveryService) Discover(ctx context.Context, index *int, policy DiscoveryPolicy) ([]DiscoveredDevice, error) {
	if ctx == nil || s.Sessions == nil {
		return nil, errors.New("discovery session dependencies are incomplete")
	}
	if err := validateDiscoveryPolicy(policy); err != nil {
		return nil, err
	}
	if policy.CycleTimeout <= 0 {
		return nil, errors.New("discovery snapshot requires a bounded policy")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	session, err := s.Sessions.Start(ctx, index, policy)
	if err != nil {
		return nil, err
	}
	if session == nil || session.Events() == nil || session.Errors() == nil {
		return nil, errors.New("discovery session channels are incomplete")
	}
	events, errs := session.Events(), session.Errors()
	devices := make(map[string]DiscoveredDevice)
	var discoveryErr error
	for events != nil || errs != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case device, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			parsed, ok := toParserDevice(device)
			if !ok {
				discoveryErr = errors.Join(discoveryErr, errors.New("discovery session returned an invalid device"))
				continue
			}
			device.MAC = parsed.MAC.String()
			key := device.MAC + "|" + parsed.Sender.String()
			devices[key] = device
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			if err == nil {
				err = errors.New("discovery session returned an empty error")
			}
			discoveryErr = errors.Join(discoveryErr, err)
		}
	}
	keys := make([]string, 0, len(devices))
	for key := range devices {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]DiscoveredDevice, 0, len(keys))
	for _, key := range keys {
		result = append(result, devices[key])
	}
	return result, discoveryErr
}
