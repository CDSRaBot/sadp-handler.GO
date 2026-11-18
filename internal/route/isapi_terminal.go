package route

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const isapiXMLNamespace = "http://www.isapi.org/ver20/XMLSchema"

type ISAPITerminalDiscovery struct {
	ONVIFDiscoveryMode *string `json:"onvif_discovery_mode,omitempty"`
	ISDiscoveryMode    *string `json:"is_discovery_mode,omitempty"`
	PcapMode           *string `json:"pcap_mode,omitempty"`
	SADP               *string `json:"sadp,omitempty"`
}

func ValidateISAPITerminalID(id string) error {
	if !validISAPIInterfaceID(id) || id == "capabilities" {
		return errors.New("invalid ISAPI terminal ID")
	}
	return nil
}

// ReadTerminalDiscovery reads documented terminal settings after authenticating
// the device and matching its MAC. It does not imply mutation support.
func (t *ISAPIHTTPTransport) ReadTerminalDiscovery(ctx context.Context, target Target, terminalID string) (ISAPITerminalDiscovery, error) {
	if ctx == nil || t == nil || t.Username == "" || t.Password == "" ||
		len(target.MAC) != 6 || target.Interface != "" || ValidateISAPITerminalID(terminalID) != nil {
		return ISAPITerminalDiscovery{}, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI terminal status arguments are invalid")}
	}
	if err := ctx.Err(); err != nil {
		return ISAPITerminalDiscovery{}, contextError(err)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if _, err := t.discover(ctx, target); err != nil {
		return ISAPITerminalDiscovery{}, err
	}
	base, err := t.baseURL(target)
	if err != nil {
		return ISAPITerminalDiscovery{}, err
	}
	endpoint := base.ResolveReference(&url.URL{Path: "/ISAPI/Publish/TerminalMgr/terminals/discovery/" + terminalID})
	payload, err := t.requestWithPolicy(ctx, http.MethodGet, endpoint, nil, true)
	if err != nil {
		return ISAPITerminalDiscovery{}, err
	}
	result, err := parseISAPITerminalDiscovery(payload)
	if err != nil {
		return ISAPITerminalDiscovery{}, &Error{Kind: ErrorMalformed, Err: errors.New("ISAPI terminal discovery response is malformed")}
	}
	return result, nil
}

func parseISAPITerminalDiscovery(payload []byte) (ISAPITerminalDiscovery, error) {
	var result ISAPITerminalDiscovery
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	seen := make(map[string]bool)
	depth := 0
	rootSeen := false
	fields := 0
	invalid := func() (ISAPITerminalDiscovery, error) {
		return ISAPITerminalDiscovery{}, errors.New("invalid terminal discovery response")
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			if rootSeen && depth == 0 && fields != 0 {
				return result, nil
			}
			return invalid()
		}
		if err != nil {
			return invalid()
		}
		switch token := token.(type) {
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(token)) != "" {
				return invalid()
			}
		case xml.EndElement:
			depth--
		case xml.StartElement:
			depth++
			if depth == 1 {
				if rootSeen || token.Name.Local != "DiscoveryMode" || token.Name.Space != isapiXMLNamespace {
					return invalid()
				}
				rootSeen = true
				version := ""
				versionSeen := false
				for _, attr := range token.Attr {
					if attr.Name.Space == "" && attr.Name.Local == "version" {
						if versionSeen {
							return invalid()
						}
						versionSeen = true
						version = attr.Value
					}
				}
				if version != "2.0" {
					return invalid()
				}
				continue
			}
			if depth != 2 {
				continue
			}
			var destination **string
			switch token.Name.Local {
			case "ONVIFDiscoveryMode":
				destination = &result.ONVIFDiscoveryMode
			case "ISDiscoveryMode":
				destination = &result.ISDiscoveryMode
			case "PcapMode":
				destination = &result.PcapMode
			case "SADP":
				destination = &result.SADP
			default:
				continue
			}
			if token.Name.Space != isapiXMLNamespace || seen[token.Name.Local] {
				return invalid()
			}
			seen[token.Name.Local] = true
			value, err := decodeSimpleElement(decoder)
			if err != nil {
				return invalid()
			}
			value = strings.TrimSpace(value)
			if value != "discoverable" && value != "nonDiscoverable" {
				return invalid()
			}
			*destination = &value
			fields++
			depth--
		}
	}
}
