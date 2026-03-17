package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/config"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/models"
)

type WalletService interface {
	CreateNativeCharge(ctx context.Context, req models.BalanceChargeRequest) (*models.OnchainRecordResponse, error)
}

type walletService struct {
	nonce   SignerNonceService
	onchain OnchainRecordService
}

func NewWalletService(onchain OnchainRecordService) WalletService {
	return &walletService{
		nonce:   NewSignerNonceService(),
		onchain: onchain,
	}
}

func (s *walletService) CreateNativeCharge(ctx context.Context, req models.BalanceChargeRequest) (*models.OnchainRecordResponse, error) {
	record, err := s.createNativeChargePrepared(ctx, req, nil)
	if err != nil {
		return nil, err
	}
	return s.onchain.CreatePrepared(*record)
}

func (s *walletService) createNativeChargePrepared(ctx context.Context, req models.BalanceChargeRequest, nonceOverride *uint64) (*PreparedOnchainRecord, error) {
	walletCfg, networkCfg, err := findWalletConfig()
	if err != nil {
		return nil, err
	}
	networkCode := networkCfg.Code

	client, err := ethclient.DialContext(ctx, networkCfg.RPCURL)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(walletCfg.PrivateKey, "0x"))
	if err != nil {
		return nil, err
	}
	signerAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	if walletCfg.FromAddress != "" && !strings.EqualFold(walletCfg.FromAddress, signerAddress.Hex()) {
		return nil, errors.New("wallet fromAddress does not match privateKey")
	}

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

	tipCap, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, err
	}
	head, err := client.BlockByNumber(ctx, nil)
	if err != nil {
		return nil, err
	}
	gasFeeCap := new(big.Int).Add(new(big.Int).Mul(head.BaseFee(), big.NewInt(2)), tipCap)

	valueWei, ok := new(big.Int).SetString(req.Value.String(), 10)
	if !ok {
		return nil, fmt.Errorf("invalid value: %s", req.Value.String())
	}
	to := common.HexToAddress(req.ReceiverAddress)
	gasLimit := uint64(21000)

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(networkCfg.ChainID),
		Nonce:     nonce,
		To:        &to,
		Value:     valueWei,
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
		"value": valueWei.String(),
		"nonce": nonce,
		"data":  "",
	})

	record := PreparedOnchainRecord{
		Code:                  signedTx.Hash().Hex(),
		IdempotencyKey:        req.IdempotencyKey,
		SignerAddress:         signerAddress.Hex(),
		OnchainType:           "NATIVE_TOKEN_CHARGE",
		OnchainStatus:         "INIT",
		RequestBusiData:       mustJSON(req),
		ResponseBusiData:      mustJSON(req),
		NetworkCode:           networkCode,
		SignedTransactionData: "0x" + hex.EncodeToString(bin),
		RawTransactionData:    string(rawTx),
		TxCode:                signedTx.Hash().Hex(),
		Nonce:                 nonce,
	}
	return &record, nil
}

func findWalletConfig() (config.WalletSigner, config.NetworkConfig, error) {
	cfg := config.GetConfig()
	if cfg.Connector == nil {
		return config.WalletSigner{}, config.NetworkConfig{}, errors.New("wallet config not found")
	}
	network, err := configuredNetwork()
	if err != nil {
		return config.WalletSigner{}, config.NetworkConfig{}, err
	}
	if cfg.Connector.Wallet.PrivateKey == "" {
		return config.WalletSigner{}, config.NetworkConfig{}, errors.New("wallet config not found")
	}
	return cfg.Connector.Wallet, network, nil
}

func mustJSON(v interface{}) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
