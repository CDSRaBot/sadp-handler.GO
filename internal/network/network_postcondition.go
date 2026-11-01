package network

import (
	"bytes"
	"context"
	"net/netip"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
)

// VerifyNetwork performs the required post-condition inquiry after a network
// mutation. Device MAC remains the identity while sender and IPv4 may change.
func (t SADPMutationTransport) VerifyNetwork(ctx context.Context, target logic.MutationTarget, config logic.NetworkConfig) error {
	_, err := t.ExchangeMatching(ctx, target, logic.BuildInquiryXML(target.UUID), func(candidate logic.MutationExchange) bool {
		identity, err := logic.ParseMutationResponseIdentity(candidate.Data)
		if err != nil || identity.Types != "inquiry" || !bytes.Equal(identity.MAC, target.MAC) {
			return false
		}
		device, err := parser.ParseAdvertisement(candidate.Data, candidate.Sender, t.datagramLimit())
		if err != nil {
			return false
		}
		return networkStateMatches(device, config)
	})
	return err
}

func (t SADPMutationTransport) datagramLimit() int {
	if t.MaxDatagramSize == 0 {
		return DefaultMaxSADPDatagramSize
	}
	return t.MaxDatagramSize
}

func networkStateMatches(device parser.Device, config logic.NetworkConfig) bool {
	if config.UseDHCP {
		return device.DHCP && device.IPv4Assigned && !device.IPv4.IsUnspecified()
	}
	if !device.IPv4Assigned || device.IPv4 != config.Address.Addr() {
		return false
	}
	if config.Address.Bits() >= 0 {
		wantMask := netip.MustParseAddr(maskFromPrefix(config.Address.Bits()))
		if !device.IPv4Mask.IsValid() || device.IPv4Mask != wantMask {
			return false
		}
	}
	return device.IPv4Gateway.IsValid() && device.IPv4Gateway == config.Gateway
}

func maskFromPrefix(bits int) string {
	if bits == 32 {
		return "255.255.255.255"
	}
	value := uint32(0xffffffff) << (32 - bits)
	return netip.AddrFrom4([4]byte{byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value)}).String()
}

var _ logic.NetworkPostConditionVerifier = SADPMutationTransport{}
