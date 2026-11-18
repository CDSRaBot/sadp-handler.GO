package route

import (
	"net/netip"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

func TestISAPINetworkStaticHostAndGatewayValidation(t *testing.T) {
	for _, tt := range []struct {
		address string
		gateway string
		valid   bool
	}{
		{"198.51.100.20/24", "198.51.100.1", true},
		{"198.51.100.0/24", "198.51.100.1", false},
		{"198.51.100.255/24", "198.51.100.1", false},
		{"198.51.100.20/24", "198.51.100.0", false},
		{"198.51.100.20/24", "198.51.100.255", false},
		{"198.51.100.20/31", "198.51.100.21", true},
		{"198.51.100.20/32", "198.51.100.20", false},
		{"198.51.100.20/24", "203.0.113.1", false},
	} {
		plan := logic.NetworkConfig{Address: netip.MustParsePrefix(tt.address), Gateway: netip.MustParseAddr(tt.gateway)}
		if err := ValidateISAPINetworkInputs("1", plan); (err == nil) != tt.valid {
			t.Errorf("%s gateway %s: valid=%t err=%v", tt.address, tt.gateway, tt.valid, err)
		}
	}
}
