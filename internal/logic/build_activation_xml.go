package logic

import (
	"errors"
	"net"
)

type ActivationRequest struct {
	MAC      net.HardwareAddr
	Password string
	Email    string
}

func BuildActivationXML(request ActivationRequest) ([]byte, error) {
	return nil, errors.New("not implemented")
}
