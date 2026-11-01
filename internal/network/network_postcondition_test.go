package network

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

type postconditionRead struct {
	data   []byte
	sender net.Addr
}

type postconditionConn struct {
	reads  []postconditionRead
	closed bool
}

func (c *postconditionConn) ReadFrom(buffer []byte) (int, net.Addr, error) {
	if len(c.reads) == 0 {
		return 0, nil, postconditionTimeout{}
	}
	next := c.reads[0]
	c.reads = c.reads[1:]
	copy(buffer, next.data)
	return len(next.data), next.sender, nil
}

func (c *postconditionConn) WriteTo(data []byte, _ net.Addr) (int, error) {
	return len(data), nil
}

func (c *postconditionConn) Close() error {
	c.closed = true
	return nil
}

func (*postconditionConn) LocalAddr() net.Addr              { return &net.UDPAddr{} }
func (*postconditionConn) SetDeadline(time.Time) error      { return nil }
func (*postconditionConn) SetReadDeadline(time.Time) error  { return nil }
func (*postconditionConn) SetWriteDeadline(time.Time) error { return nil }

type postconditionTimeout struct{}

func (postconditionTimeout) Error() string   { return "timeout" }
func (postconditionTimeout) Timeout() bool   { return true }
func (postconditionTimeout) Temporary() bool { return true }

func TestVerifyNetworkAcceptsFinalInquiryFromNewSender(t *testing.T) {
	target := logic.MutationTarget{
		MAC:    net.HardwareAddr{0, 17, 34, 51, 68, 85},
		Sender: &net.UDPAddr{IP: net.ParseIP("192.0.2.10"), Port: 37020},
	}
	conn := &postconditionConn{reads: []postconditionRead{
		{
			data:   []byte(`<ProbeMatch><Uuid>wrong</Uuid><Types>inquiry</Types><MAC>aa:bb:cc:dd:ee:ff</MAC><IPv4Address>192.0.2.20</IPv4Address></ProbeMatch>`),
			sender: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 37020},
		},
		{
			data:   []byte(`<ProbeMatch><Uuid>final</Uuid><Types>inquiry</Types><MAC>00:11:22:33:44:55</MAC><IPv4Address>192.0.2.20</IPv4Address><IPv4SubnetMask>255.255.255.0</IPv4SubnetMask><IPv4Gateway>192.0.2.1</IPv4Gateway><DHCP>false</DHCP></ProbeMatch>`),
			sender: &net.UDPAddr{IP: net.ParseIP("192.0.2.20"), Port: 37020},
		},
	}}
	transport := SADPMutationTransport{MaxDatagramSize: 1024, Open: func() (net.PacketConn, error) {
		return conn, nil
	}}
	err := transport.VerifyNetwork(context.Background(), target, logic.NetworkConfig{
		Address: netip.MustParsePrefix("192.0.2.20/24"),
		Gateway: netip.MustParseAddr("192.0.2.1"),
	})
	if err != nil {
		t.Fatalf("VerifyNetwork() error = %v", err)
	}
	if !conn.closed {
		t.Fatal("post-condition connection was not closed")
	}
}

func TestExchangeMatchingDrainsUnrelatedDatagrams(t *testing.T) {
	target := logic.MutationTarget{
		MAC:    net.HardwareAddr{0, 17, 34, 51, 68, 85},
		Sender: &net.UDPAddr{IP: net.ParseIP("192.0.2.10"), Port: 37020},
	}
	conn := &postconditionConn{reads: []postconditionRead{
		{data: []byte(`<ProbeMatch><Types>inquiry</Types><MAC>aa-bb-cc-dd-ee-ff</MAC></ProbeMatch>`), sender: target.Sender},
		{data: []byte(`<ProbeMatch><Types>update</Types><MAC>00-11-22-33-44-55</MAC></ProbeMatch>`), sender: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 37020}},
	}}
	transport := SADPMutationTransport{MaxDatagramSize: 1024, Open: func() (net.PacketConn, error) {
		return conn, nil
	}}
	exchange, err := transport.ExchangeMatching(context.Background(), target, []byte("request"), func(candidate logic.MutationExchange) bool {
		identity, parseErr := logic.ParseMutationResponseIdentity(candidate.Data)
		return parseErr == nil && identity.Types == "update" && identity.MAC.String() == target.MAC.String()
	})
	if err != nil {
		t.Fatalf("ExchangeMatching() error = %v", err)
	}
	if string(exchange.Data) == "" || !conn.closed {
		t.Fatalf("exchange/cleanup = %#v/%t", exchange, conn.closed)
	}
}

func TestVerifyNetworkFailsWhenFinalStateIsAbsent(t *testing.T) {
	target := logic.MutationTarget{
		MAC:    net.HardwareAddr{0, 17, 34, 51, 68, 85},
		Sender: &net.UDPAddr{IP: net.ParseIP("192.0.2.10"), Port: 37020},
	}
	transport := SADPMutationTransport{MaxDatagramSize: 1024, Open: func() (net.PacketConn, error) {
		return &postconditionConn{}, nil
	}}
	err := transport.VerifyNetwork(context.Background(), target, logic.NetworkConfig{UseDHCP: true})
	var transportErr *TransportError
	if !errors.As(err, &transportErr) || transportErr.Stage != TransportTimeout {
		t.Fatalf("VerifyNetwork() error = %v, want transport timeout", err)
	}
}
