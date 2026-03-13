package service

import (
	"sync"
	"time"

	"github.com/guyuxiang/projectc-ethereum-connector/pkg/mysql"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SignerNonceService interface {
	GetAndIncrementNonce(signerAddress, networkCode string, fetchChainNonce func() (uint64, error)) (uint64, error)
	ResetNextNonce(signerAddress, networkCode string, nextNonce uint64) error
}

type signerNonceService struct {
	mu      sync.Mutex
	memData map[string]uint64
}

func NewSignerNonceService() SignerNonceService {
	return &signerNonceService{memData: map[string]uint64{}}
}

func (s *signerNonceService) GetAndIncrementNonce(signerAddress, networkCode string, fetchChainNonce func() (uint64, error)) (uint64, error) {
	if mysql.DB() != nil {
		return s.getAndIncrementNonceDB(signerAddress, networkCode, fetchChainNonce)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := signerAddress + ":" + networkCode
	if current, ok := s.memData[key]; ok {
		s.memData[key] = current + 1
		return current + 1, nil
	}

	chainNonce, err := fetchChainNonce()
	if err != nil {
		return 0, err
	}
	s.memData[key] = chainNonce
	return chainNonce, nil
}

func (s *signerNonceService) getAndIncrementNonceDB(signerAddress, networkCode string, fetchChainNonce func() (uint64, error)) (uint64, error) {
	var nonce uint64
	err := mysql.DB().Transaction(func(tx *gorm.DB) error {
		var row store.SignerNoncePO
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("signer_address = ? and network_code = ?", signerAddress, networkCode).
			First(&row).Error
		if err != nil {
			chainNonce, fetchErr := fetchChainNonce()
			if fetchErr != nil {
				return fetchErr
			}
			row = store.SignerNoncePO{
				SignerAddress: signerAddress,
				NetworkCode:   networkCode,
				CurrentNonce:  chainNonce,
				SyncTimestamp: time.Now().UnixMilli(),
			}
			if createErr := tx.Create(&row).Error; createErr != nil {
				return createErr
			}
			nonce = chainNonce
			return nil
		}

		row.CurrentNonce++
		row.SyncTimestamp = time.Now().UnixMilli()
		if saveErr := tx.Save(&row).Error; saveErr != nil {
			return saveErr
		}
		nonce = row.CurrentNonce
		return nil
	})
	return nonce, err
}

func (s *signerNonceService) ResetNextNonce(signerAddress, networkCode string, nextNonce uint64) error {
	if nextNonce == 0 {
		return nil
	}
	if mysql.DB() != nil {
		return s.resetNextNonceDB(signerAddress, networkCode, nextNonce)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.memData[signerAddress+":"+networkCode] = nextNonce - 1
	return nil
}

func (s *signerNonceService) resetNextNonceDB(signerAddress, networkCode string, nextNonce uint64) error {
	return mysql.DB().Transaction(func(tx *gorm.DB) error {
		var row store.SignerNoncePO
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("signer_address = ? and network_code = ?", signerAddress, networkCode).
			First(&row).Error
		if err != nil {
			row = store.SignerNoncePO{
				SignerAddress: signerAddress,
				NetworkCode:   networkCode,
				CurrentNonce:  nextNonce - 1,
				SyncTimestamp: time.Now().UnixMilli(),
			}
			return tx.Create(&row).Error
		}

		row.CurrentNonce = nextNonce - 1
		row.SyncTimestamp = time.Now().UnixMilli()
		return tx.Save(&row).Error
	})
}
