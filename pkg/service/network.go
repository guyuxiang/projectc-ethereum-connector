package service

import (
	"errors"

	"github.com/guyuxiang/projectc-ethereum-connector/pkg/config"
)

func configuredNetwork() (config.NetworkConfig, error) {
	cfg := config.GetConfig()
	if cfg.Network == nil {
		return config.NetworkConfig{}, errors.New("network config not found")
	}
	if cfg.Network.Rpcurl == "" {
		return config.NetworkConfig{}, errors.New("network rpcUrl is required")
	}
	if cfg.Network.Networkcode == "" {
		return config.NetworkConfig{}, errors.New("network code is required")
	}
	return *cfg.Network, nil
}

func configuredNetworkCode() string {
	network, err := configuredNetwork()
	if err != nil {
		return ""
	}
	return network.Networkcode
}

func ConfiguredNetworkCode() string {
	return configuredNetworkCode()
}
