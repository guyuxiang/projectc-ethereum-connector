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
	CreateNativeCharge(ctx context.Context, networkCode string, req models.BalanceChargeRequest) (*models.OnchainRecordResponse, error)
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

func (s *walletService) CreateNativeCharge(ctx context.Context, networkCode string, req models.BalanceChargeRequest) (*models.OnchainRecordResponse, error) {
	record, err := s.createNativeChargePrepared(ctx, networkCode, req, nil)
	if err != nil {
		return nil, err
	}
	return s.onchain.CreatePrepared(*record)
}

func (s *walletService) createNativeChargePrepared(ctx context.Context, networkCode string, req models.BalanceChargeRequest, nonceOverride *uint64) (*PreparedOnchainRecord, error) {
	walletCfg, networkCfg, err := findWalletConfig(networkCode)
	if err != nil {
		return nil, err
	}

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

func findWalletConfig(networkCode string) (config.WalletSigner, config.NetworkConfig, error) {
	cfg := config.GetConfig()
	var walletCfg config.WalletSigner
	foundWallet := false
	if cfg.Connector != nil {
		for _, wallet := range cfg.Connector.Wallets {
			if wallet.NetworkCode == networkCode {
				walletCfg = wallet
				foundWallet = true
				break
			}
		}
	}
	if !foundWallet {
		return config.WalletSigner{}, config.NetworkConfig{}, errors.New("wallet config not found")
	}
	if cfg.Ethereum != nil {
		for _, network := range cfg.Ethereum.Networks {
			if network.Code == networkCode {
				return walletCfg, network, nil
			}
		}
	}
	return config.WalletSigner{}, config.NetworkConfig{}, errors.New("network config not found")
}

func mustJSON(v interface{}) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
