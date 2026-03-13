package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/config"
)

type contractTxService struct {
	nonce     SignerNonceService
	contracts ContractRegistryService
}

func newContractTxService(contracts ContractRegistryService) *contractTxService {
	return &contractTxService{
		nonce:     NewSignerNonceService(),
		contracts: contracts,
	}
}

func (s *contractTxService) createPreparedContractTx(ctx context.Context, networkCode, onchainType, idempotencyKey string, requestData, responseData interface{}, abiJSON, method string, args ...interface{}) (*PreparedOnchainRecord, error) {
	return s.createPreparedContractTxWithNonce(ctx, networkCode, onchainType, idempotencyKey, requestData, responseData, abiJSON, method, nil, args...)
}

func (s *contractTxService) createPreparedContractTxWithNonce(ctx context.Context, networkCode, onchainType, idempotencyKey string, requestData, responseData interface{}, abiJSON, method string, nonceOverride *uint64, args ...interface{}) (*PreparedOnchainRecord, error) {
	signerCfg, networkCfg, err := findOnchainSignerConfig(networkCode, onchainType)
	if err != nil {
		return nil, err
	}
	contract, err := s.contracts.FindContract(networkCode, signerCfg.ContractCode)
	if err != nil {
		return nil, err
	}
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, err
	}
	data, err := parsedABI.Pack(method, args...)
	if err != nil {
		return nil, err
	}

	client, err := ethclient.DialContext(ctx, networkCfg.RPCURL)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(signerCfg.PrivateKey, "0x"))
	if err != nil {
		return nil, err
	}
	signerAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	if nonceOverride != nil {
		if err = s.nonce.ResetNextNonce(signerAddress.Hex(), networkCode, *nonceOverride); err != nil {
			return nil, err
		}
	}

	nonce, err := s.nonce.GetAndIncrementNonce(signerAddress.Hex(), networkCode, func() (uint64, error) {
		return client.PendingNonceAt(ctx, signerAddress)
	})
	if err != nil {
		return nil, err
	}

	to := common.HexToAddress(contract.Address)
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From: signerAddress,
		To:   &to,
		Data: data,
	})
	if err != nil {
		gasLimit = 500000
	}
	tipCap, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, err
	}
	head, err := client.BlockByNumber(ctx, nil)
	if err != nil {
		return nil, err
	}
	gasFeeCap := new(big.Int).Add(new(big.Int).Mul(head.BaseFee(), big.NewInt(2)), tipCap)

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(networkCfg.ChainID),
		Nonce:     nonce,
		To:        &to,
		Value:     big.NewInt(0),
		Data:      data,
		Gas:       gasLimit,
		GasTipCap: tipCap,
		GasFeeCap: gasFeeCap,
	})
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(networkCfg.ChainID)), privateKey)
	if err != nil {
		return nil, err
	}
	bin, err := signedTx.MarshalBinary()
	if err != nil {
		return nil, err
	}
	rawTx, _ := json.Marshal(map[string]interface{}{
		"to":    to.Hex(),
		"value": "0",
		"nonce": nonce,
		"data":  "0x" + hex.EncodeToString(data),
	})

	return &PreparedOnchainRecord{
		Code:                  signedTx.Hash().Hex(),
		IdempotencyKey:        idempotencyKey,
		SignerAddress:         signerAddress.Hex(),
		OnchainType:           onchainType,
		OnchainStatus:         "INIT",
		RequestBusiData:       mustJSON(requestData),
		ResponseBusiData:      mustJSON(responseData),
		NetworkCode:           networkCode,
		SignedTransactionData: "0x" + hex.EncodeToString(bin),
		RawTransactionData:    string(rawTx),
		TxCode:                signedTx.Hash().Hex(),
		Nonce:                 nonce,
	}, nil
}

func findOnchainSignerConfig(networkCode, onchainType string) (config.OnchainSigner, config.NetworkConfig, error) {
	cfg := config.GetConfig()
	var signer config.OnchainSigner
	found := false
	if cfg.Connector != nil {
		for _, item := range cfg.Connector.Onchains {
			if item.NetworkCode == networkCode && item.OnchainType == onchainType {
				signer = item
				found = true
				break
			}
		}
	}
	if !found {
		return config.OnchainSigner{}, config.NetworkConfig{}, errors.New("onchain signer config not found")
	}
	network, err := findNetworkConfig(networkCode)
	if err != nil {
		return config.OnchainSigner{}, config.NetworkConfig{}, err
	}
	return signer, network, nil
}
