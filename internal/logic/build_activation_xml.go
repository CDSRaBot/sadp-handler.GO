package logic

import (
	"errors"
	"net"
	"net/netip"
)

type ActivationRequest struct {
	MAC      net.HardwareAddr
	Password string
	Email    string
}

type NetworkConfig struct {
	Address netip.Prefix
	Gateway netip.Addr
	UseDHCP bool
}

func BuildActivationXML(request ActivationRequest) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func BuildNetworkXML(mac net.HardwareAddr, config NetworkConfig, challenge string, password string) ([]byte, error) {
	return nil, errors.New("not implemented")
}
