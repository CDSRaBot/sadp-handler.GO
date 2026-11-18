package ui

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/netip"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

// ISAPIDHCPNetworkLocator treats SADP advertisements as address candidates,
// never as authenticated proof of identity.
type ISAPIDHCPNetworkLocator struct {
	Sessions       DiscoverySessionService
	InterfaceIndex int
	Policy         DiscoveryPolicy
	Transport      *route.ISAPIHTTPTransport
}

func (l ISAPIDHCPNetworkLocator) Await(ctx context.Context, target route.Target, plan logic.NetworkConfig) (route.Target, error) {
	if ctx == nil || l.Sessions == nil || l.InterfaceIndex < 1 || l.Transport == nil ||
		l.Transport.Username == "" || l.Transport.Password == "" || len(target.MAC) != 6 ||
		target.Interface != "" || !plan.UseDHCP || logic.ValidateNetworkConfig(plan) != nil ||
		validateDiscoveryPolicy(l.Policy) != nil || l.Policy.ProbeInterval <= 0 {
		return route.Target{}, &route.Error{Kind: route.ErrorInvalidInput, Err: errors.New("ISAPI DHCP locator arguments are invalid")}
	}
	if err := ctx.Err(); err != nil {
		return route.Target{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	session, err := l.Sessions.Start(ctx, &l.InterfaceIndex, l.Policy)
	if err != nil {
		return route.Target{}, err
	}
	if session == nil || session.Events() == nil || session.Errors() == nil {
		return route.Target{}, &route.Error{Kind: route.ErrorTransport, Err: errors.New("DHCP discovery session channels are incomplete")}
	}
	events, errs := session.Events(), session.Errors()
	for events != nil || errs != nil {
		select {
		case <-ctx.Done():
			return route.Target{}, ctx.Err()
		case discoveryErr, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			if discoveryErr == nil {
				discoveryErr = errors.New("DHCP discovery reported an empty error")
			}
			return route.Target{}, discoveryErr
		case device, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			mac, parseErr := net.ParseMAC(device.MAC)
			if parseErr != nil || len(mac) != 6 {
				return route.Target{}, &route.Error{Kind: route.ErrorMalformed, Err: errors.New("DHCP discovery returned an invalid identity")}
			}
			if !bytes.Equal(mac, target.MAC) {
				continue
			}
			address, parseErr := netip.ParseAddr(device.IPv4)
			if parseErr != nil || !address.Is4() || !address.IsGlobalUnicast() {
				return route.Target{}, &route.Error{Kind: route.ErrorMalformed, Err: errors.New("DHCP discovery returned an invalid address")}
			}
			candidate := target
			candidate.Address = address
			_, verifyErr := l.Transport.Execute(ctx, route.OperationDiscovery, candidate)
			if verifyErr == nil {
				return candidate, nil
			}
			if ctx.Err() != nil {
				return route.Target{}, ctx.Err()
			}
			var routeErr *route.Error
			if errors.As(verifyErr, &routeErr) && (routeErr.Kind == route.ErrorTransport || routeErr.Kind == route.ErrorTimeout || routeErr.Kind == route.ErrorNoRoute) {
				continue
			}
			return route.Target{}, verifyErr
		}
	}
	return route.Target{}, &route.Error{Kind: route.ErrorVerification, Err: errors.New("DHCP discovery ended before the device reappeared")}
}
