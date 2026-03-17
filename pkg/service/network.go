package service

import (
	"errors"

	"github.com/guyuxiang/projectc-ethereum-connector/pkg/config"
)

func configuredNetwork() (config.NetworkConfig, error) {
	cfg := config.GetConfig()
	if cfg.Ethereum == nil {
		return config.NetworkConfig{}, errors.New("ethereum config not found")
	}
	if cfg.Ethereum.Network.RPCURL == "" {
		return config.NetworkConfig{}, errors.New("ethereum network rpcUrl is required")
	}
	if cfg.Ethereum.Network.Code == "" {
		return config.NetworkConfig{}, errors.New("ethereum network code is required")
	}
	return cfg.Ethereum.Network, nil
}

func configuredNetworkCode() string {
	network, err := configuredNetwork()
	if err != nil {
		return ""
	}
	return network.Code
}

func ConfiguredNetworkCode() string {
	return configuredNetworkCode()
}
