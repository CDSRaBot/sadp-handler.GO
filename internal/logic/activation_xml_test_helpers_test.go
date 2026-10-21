package logic_test

import (
	"encoding/xml"
	"testing"
)

type activationXML struct {
	XMLName    xml.Name `xml:"Activate"`
	TargetMAC  string   `xml:"TargetMAC"`
	Password   string   `xml:"Password"`
	OptionData *struct {
		SecurityEmail string `xml:"SecurityEmail"`
	} `xml:"OptionData"`
}

func parseActivationXML(t *testing.T, payload []byte) activationXML {
	t.Helper()

	var document activationXML
	if err := xml.Unmarshal(payload, &document); err != nil {
		t.Fatalf("XML is invalid: %v\npayload: %q", err, payload)
	}
	return document
}
