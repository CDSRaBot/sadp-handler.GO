package ui

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
	"github.com/CDSRaBot/sadp-handler.GO/internal/discovery"
	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
)

type M6DiscoverySessionService struct {
	ListInterfaces func() ([]adapter.Interface, error)
	Discover       func(context.Context, adapter.Interface, discovery.DiscoveryOptions, discovery.EventHandler) error
	Multicast      adapter.MulticastOptions
}

func NewM6DiscoverySessionService() M6DiscoverySessionService {
	return M6DiscoverySessionService{
		ListInterfaces: adapter.ListInterfaces,
		Discover:       discovery.DiscoverOnInterface,
		Multicast: adapter.MulticastOptions{
			Group: netip.MustParseAddrPort("239.255.255.250:37020"),
			TTL:   1,
		},
	}
}

func (s M6DiscoverySessionService) ListEligibleInterfaces() ([]adapter.Interface, error) {
	if s.ListInterfaces == nil {
		return nil, errors.New("interface listing dependency is incomplete")
	}
	interfaces, err := s.ListInterfaces()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}
	return EligibleInterfaces(interfaces), nil
}

func (s M6DiscoverySessionService) Start(ctx context.Context, index *int, policy DiscoveryPolicy) (DiscoverySession, error) {
	if ctx == nil {
		return nil, errors.New("discovery context is nil")
	}
	if index == nil || *index < 1 {
		return nil, errors.New("an eligible interface index is required")
	}
	if s.ListInterfaces == nil || s.Discover == nil {
		return nil, errors.New("discovery service dependencies are incomplete")
	}
	if err := validateDiscoveryPolicy(policy); err != nil {
		return nil, fmt.Errorf("validate discovery policy: %w", err)
	}
	if !s.Multicast.Group.IsValid() {
		return nil, errors.New("multicast group is invalid")
	}
	interfaces, err := s.ListInterfaces()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}
	selected, err := ResolveInterface(interfaces, *index)
	if err != nil {
		return nil, err
	}

	session := &m6DiscoverySession{
		events: make(chan DiscoveredDevice, 16),
		errs:   make(chan error, 1),
	}
	options := discovery.DiscoveryOptions{
		Multicast:           s.Multicast,
		ProbeInterval:       policy.ProbeInterval,
		CycleTimeout:        policy.CycleTimeout,
		RepeatCount:         policy.RepeatCount,
		MaxSADPDatagramSize: policy.MaxSADPDatagramSize,
	}
	if err := validateDiscoveryOptions(options); err != nil {
		return nil, err
	}
	go session.run(ctx, s.Discover, selected, options)
	return session, nil
}

type m6DiscoverySession struct {
	events chan DiscoveredDevice
	errs   chan error
}

func (s *m6DiscoverySession) Events() <-chan DiscoveredDevice {
	return s.events
}

func (s *m6DiscoverySession) Errors() <-chan error {
	return s.errs
}

func (s *m6DiscoverySession) run(ctx context.Context, discover func(context.Context, adapter.Interface, discovery.DiscoveryOptions, discovery.EventHandler) error, iface adapter.Interface, options discovery.DiscoveryOptions) {
	defer close(s.events)
	defer close(s.errs)
	handle := func(eventCtx context.Context, event discovery.Event) error {
		if event.Kind != discovery.DeviceFound || event.Device == nil {
			return nil
		}
		device, ok := discoveredDeviceFromParser(*event.Device)
		if !ok {
			return nil
		}
		select {
		case s.events <- device:
			return nil
		case <-eventCtx.Done():
			return eventCtx.Err()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := discover(ctx, iface, options, handle); err != nil && !errors.Is(err, context.Canceled) {
		s.errs <- err
	}
}

func discoveredDeviceFromParser(device parser.Device) (DiscoveredDevice, bool) {
	if len(device.MAC) != 6 || !device.IPv4.IsValid() || !device.IPv4.Is4() || device.Sender == nil {
		return DiscoveredDevice{}, false
	}
	return DiscoveredDevice{
		MAC:             device.MAC.String(),
		Sender:          device.Sender.String(),
		IPv4:            device.IPv4.String(),
		Model:           device.Model,
		Activated:       device.Activated,
		SoftwareVersion: device.SoftwareVersion,
	}, true
}

func validateDiscoveryOptions(options discovery.DiscoveryOptions) error {
	if !options.Multicast.Group.IsValid() || !options.Multicast.Group.Addr().Is4() ||
		!options.Multicast.Group.Addr().IsMulticast() || options.Multicast.Group.Port() == 0 ||
		options.Multicast.TTL < 1 || options.Multicast.TTL > 255 {
		return errors.New("multicast options are invalid")
	}
	return nil
}
