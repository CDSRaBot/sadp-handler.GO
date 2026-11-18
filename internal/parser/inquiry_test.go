package parser_test

import (
	"errors"
	"net"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
)

func TestInquiryIsNotAnAdvertisement(t *testing.T) {
	sender := &net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 37020}
	for _, tt := range []struct {
		payload string
		kind    parser.ParseErrorKind
	}{
		{`<Probe><Uuid>synthetic</Uuid><Types>inquiry</Types></Probe>`, parser.ParseInquiry},
		{`<Probe><Types>update</Types></Probe>`, parser.ParseUnsupported},
		{`<Probe><Types>inquiry</Types>`, parser.ParseMalformedXML},
		{`<Other><Types>inquiry</Types></Other>`, parser.ParseUnsupported},
	} {
		_, err := parser.ParseAdvertisement([]byte(tt.payload), sender, 1500)
		var parseErr *parser.ParseError
		if !errors.As(err, &parseErr) || parseErr.Kind != tt.kind {
			t.Fatalf("payload=%q err=%v, want %s", tt.payload, err, tt.kind)
		}
	}
}
