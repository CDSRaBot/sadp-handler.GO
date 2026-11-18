package ui

import "context"

type tuiForwardedSession struct {
	events chan DiscoveredDevice
	errs   chan error
}

func (s *tuiForwardedSession) Events() <-chan DiscoveredDevice { return s.events }
func (s *tuiForwardedSession) Errors() <-chan error            { return s.errs }

func newTUIForwardedSession(ctx context.Context, source DiscoverySession, done chan struct{}) DiscoverySession {
	result := &tuiForwardedSession{events: make(chan DiscoveredDevice, 16), errs: make(chan error, 1)}
	go func() {
		defer close(done)
		defer close(result.events)
		defer close(result.errs)
		events, errs := source.Events(), source.Errors()
		for events != nil || errs != nil {
			select {
			case event, ok := <-events:
				if !ok {
					events = nil
					continue
				}
				select {
				case result.events <- event:
				case <-ctx.Done():
				}
			case err, ok := <-errs:
				if !ok {
					errs = nil
					continue
				}
				select {
				case result.errs <- err:
				case <-ctx.Done():
				}
			}
		}
	}()
	return result
}
