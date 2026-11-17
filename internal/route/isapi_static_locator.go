package route

import (
	"context"
	"errors"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

// ISAPIStaticNetworkLocator checks the requested static address until the
// authenticated device identity is reachable again. It cannot locate DHCP.
type ISAPIStaticNetworkLocator struct {
	Transport     *ISAPIHTTPTransport
	RetryInterval time.Duration
}

func (l ISAPIStaticNetworkLocator) Await(ctx context.Context, target Target, plan logic.NetworkConfig) (Target, error) {
	if ctx == nil || l.Transport == nil || len(target.MAC) != 6 || target.Interface != "" ||
		l.Transport.Username == "" || l.Transport.Password == "" || l.RetryInterval < 0 ||
		plan.UseDHCP || !validISAPINetworkPlan(plan) {
		return Target{}, &Error{Kind: ErrorInvalidInput, Err: errors.New("static ISAPI locator arguments are invalid")}
	}
	target.Address = plan.Address.Addr()
	interval := l.RetryInterval
	if interval == 0 {
		interval = time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	for {
		_, err := l.Transport.discover(ctx, target)
		if err == nil {
			return target, nil
		}
		if ctx.Err() != nil {
			return Target{}, contextError(ctx.Err())
		}
		var routeErr *Error
		if !errors.As(err, &routeErr) || (routeErr.Kind != ErrorTransport && routeErr.Kind != ErrorTimeout && routeErr.Kind != ErrorNoRoute) {
			return Target{}, err
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Target{}, contextError(ctx.Err())
		case <-timer.C:
		}
	}
}
