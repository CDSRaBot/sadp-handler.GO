package logic_test

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

type matchingIdentityTransport struct {
	candidates [][]byte
}

func (t matchingIdentityTransport) Exchange(context.Context, logic.MutationTarget, []byte) (logic.MutationExchange, error) {
	return logic.MutationExchange{}, nil
}

func (t matchingIdentityTransport) ExchangeMatching(_ context.Context, _ logic.MutationTarget, _ []byte, matcher func(logic.MutationExchange) bool) (logic.MutationExchange, error) {
	for _, data := range t.candidates {
		candidate := logic.MutationExchange{
			Sender: &net.UDPAddr{IP: net.ParseIP("169.254.10.20"), Port: 37020},
			Data:   data,
		}
		if matcher(candidate) {
			return candidate, nil
		}
	}
	return logic.MutationExchange{}, context.DeadlineExceeded
}

func TestParseMutationResponseIdentityExtractsSafeCorrelationFields(t *testing.T) {
	identity, err := logic.ParseMutationResponseIdentity([]byte(`<ProbeMatch><Uuid>urn:uuid:response-1</Uuid><Types>update</Types><MAC>00-11-22-33-44-55</MAC><Result>success</Result><Password>secret</Password></ProbeMatch>`))
	if err != nil {
		t.Fatalf("ParseMutationResponseIdentity returned error: %v", err)
	}
	if identity.UUID != "urn:uuid:response-1" || identity.Types != "update" ||
		identity.MAC.String() != "00:11:22:33:44:55" || identity.Result != "success" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestParseMutationResponseIdentityRejectsMissingOrUnsafeEnvelope(t *testing.T) {
	for _, payload := range []string{
		`<ProbeMatch><Types>update</Types><MAC>not-a-mac</MAC></ProbeMatch>`,
		`<Other><Types>update</Types><MAC>00:11:22:33:44:55</MAC></Other>`,
	} {
		identity, err := logic.ParseMutationResponseIdentity([]byte(payload))
		if err == nil || identity.MAC != nil {
			t.Fatalf("payload identity/error = %#v/%v, want rejection", identity, err)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatalf("error disclosed payload content: %v", err)
		}
	}
}

func TestMutationTargetRetainsMACAcrossSenderChange(t *testing.T) {
	target := logic.MutationTarget{
		MAC:  net.HardwareAddr{0, 17, 34, 51, 68, 85},
		UUID: "urn:uuid:response-1",
		Sender: &net.UDPAddr{
			IP:   net.ParseIP("192.0.2.10"),
			Port: 37020,
		},
	}
	identity, err := logic.ParseMutationResponseIdentity([]byte(`<ProbeMatch><Uuid>urn:uuid:response-1</Uuid><Types>update</Types><MAC>00:11:22:33:44:55</MAC></ProbeMatch>`))
	if err != nil {
		t.Fatal(err)
	}
	if identity.MAC.String() != target.MAC.String() {
		t.Fatalf("identity MAC = %s, target MAC = %s", identity.MAC, target.MAC)
	}
}

func TestMutationWorkflowSkipsWrongTypesAndUUIDBeforeAcceptingResponse(t *testing.T) {
	target := logic.MutationTarget{
		MAC:  net.HardwareAddr{0, 17, 34, 51, 68, 85},
		UUID: "urn:uuid:expected",
		Sender: &net.UDPAddr{
			IP:   net.ParseIP("192.0.2.10"),
			Port: 37020,
		},
	}
	workflow := logic.MutationWorkflow{
		Transport: matchingIdentityTransport{candidates: [][]byte{
			[]byte(`<ProbeMatch><Uuid>urn:uuid:wrong</Uuid><Types>inquiry</Types><MAC>00:11:22:33:44:55</MAC></ProbeMatch>`),
			[]byte(`<ProbeMatch><Uuid>urn:uuid:expected</Uuid><Types>activate</Types><MAC>00:11:22:33:44:55</MAC><Result>success</Result></ProbeMatch>`),
		}},
		Parser: fakeResponseParser{parse: func(operation logic.MutationOperation, got logic.MutationTarget, sender net.Addr, _ []byte) (logic.MutationDecision, error) {
			return logic.MutationDecision{Operation: operation, Target: got, Status: logic.MutationAccepted}, nil
		}},
	}
	decision, err := workflow.Activate(context.Background(), target, validPassword(), "")
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if decision.Status != logic.MutationAccepted {
		t.Fatalf("decision status = %q", decision.Status)
	}
}
