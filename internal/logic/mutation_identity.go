package logic

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"strings"
)

// MutationResponseIdentity contains only non-sensitive SADP correlation fields.
type MutationResponseIdentity struct {
	UUID   string
	Types  string
	MAC    net.HardwareAddr
	Result string
}

type inquiryXML struct {
	XMLName      xml.Name `xml:"Probe"`
	UUID         string   `xml:"Uuid,omitempty"`
	Types        string   `xml:"Types"`
	ResponseMode string   `xml:"ResponseMode"`
}

func BuildInquiryXML(uuid string) []byte {
	payload, _ := xml.Marshal(inquiryXML{UUID: uuid, Types: "inquiry", ResponseMode: "2"})
	return append([]byte(xml.Header), payload...)
}

// ParseMutationResponseIdentity parses the safe identity envelope of a SADP
// response without interpreting operation-specific or sensitive fields.
func ParseMutationResponseIdentity(data []byte) (MutationResponseIdentity, error) {
	var envelope struct {
		XMLName xml.Name
		UUID    string `xml:"Uuid"`
		Types   string `xml:"Types"`
		MAC     string `xml:"MAC"`
		Result  string `xml:"Result"`
	}
	if len(data) == 0 {
		return MutationResponseIdentity{}, errors.New("mutation response is empty")
	}
	if err := xml.Unmarshal(data, &envelope); err != nil {
		return MutationResponseIdentity{}, fmt.Errorf("parse mutation response identity: %w", err)
	}
	if envelope.XMLName.Local != "ProbeMatch" && envelope.XMLName.Local != "Hello" {
		return MutationResponseIdentity{}, errors.New("mutation response root is unsupported")
	}
	if strings.TrimSpace(envelope.Types) == "" {
		return MutationResponseIdentity{}, errors.New("mutation response Types is missing")
	}
	mac, err := net.ParseMAC(strings.TrimSpace(envelope.MAC))
	if err != nil || len(mac) != 6 {
		if err == nil {
			err = errors.New("MAC is not EUI-48")
		}
		return MutationResponseIdentity{}, fmt.Errorf("mutation response MAC is invalid: %w", err)
	}
	return MutationResponseIdentity{
		UUID:   strings.TrimSpace(envelope.UUID),
		Types:  strings.TrimSpace(envelope.Types),
		MAC:    append(net.HardwareAddr(nil), mac...),
		Result: strings.TrimSpace(envelope.Result),
	}, nil
}

func identityMatches(operation MutationOperation, target MutationTarget, identity MutationResponseIdentity) bool {
	if !sameMAC(identity.MAC, target.MAC) {
		return false
	}
	if target.UUID != "" && identity.UUID != target.UUID {
		return false
	}
	switch operation {
	case MutationActivation:
		return identity.Types == "activate"
	case MutationNetwork:
		return identity.Types == "update"
	case MutationSecurityQuestions:
		return identity.Types == "setsecurityquestion"
	case MutationHikConnect:
		return identity.Types == "SetHCPlatform"
	default:
		return false
	}
}
