package logic

import (
	"encoding/xml"
	"errors"
	"net"
	"strings"
)

const (
	SADPTypeSecurityQuestions = "setsecurityquestion"
	SADPTypeHikConnect        = "SetHCPlatform"
)

type SecurityQuestion struct {
	Question string
	Answer   string
}

type SecurityQuestionsRequest struct {
	MAC       net.HardwareAddr
	Questions []SecurityQuestion
}

type HikConnectRequest struct {
	MAC      net.HardwareAddr
	Enabled  bool
	ServerID string
}

func BuildSecurityQuestionsXML(request SecurityQuestionsRequest) ([]byte, error) {
	if err := validateMAC(request.MAC); err != nil {
		return nil, invalidOperation("MAC", err)
	}
	if len(request.Questions) == 0 {
		return nil, invalidOperation("security questions", errors.New("questions are optional but at least one is required when configured"))
	}
	document := sadpQuestionXML{Types: SADPTypeSecurityQuestions, MAC: strings.ToUpper(request.MAC.String())}
	for _, question := range request.Questions {
		if question.Question == "" || question.Answer == "" {
			return nil, invalidOperation("security questions", errors.New("question and answer are required"))
		}
		document.Questions = append(document.Questions, sadpQuestionPairXML{Question: question.Question, Answer: question.Answer})
	}
	return marshalXML(document)
}

func BuildHikConnectXML(request HikConnectRequest) ([]byte, error) {
	if err := validateMAC(request.MAC); err != nil {
		return nil, invalidOperation("MAC", err)
	}
	return marshalXML(sadpHikConnectXML{
		Types:    SADPTypeHikConnect,
		MAC:      strings.ToUpper(request.MAC.String()),
		Enabled:  boolString(request.Enabled),
		ServerID: request.ServerID,
	})
}

type sadpQuestionXML struct {
	XMLName   xml.Name              `xml:"Probe"`
	Types     string                `xml:"Types"`
	MAC       string                `xml:"MAC"`
	Questions []sadpQuestionPairXML `xml:"Question"`
}

type sadpQuestionPairXML struct {
	Question string `xml:"Question"`
	Answer   string `xml:"Answer"`
}

type sadpHikConnectXML struct {
	XMLName  xml.Name `xml:"Probe"`
	Types    string   `xml:"Types"`
	MAC      string   `xml:"MAC"`
	Enabled  string   `xml:"HCPlatformEnable"`
	ServerID string   `xml:"HCPlatformServerID,omitempty"`
}
