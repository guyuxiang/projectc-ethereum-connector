package service

import (
	"errors"
	"strings"
	"sync"

	"github.com/guyuxiang/projectc-ethereum-connector/pkg/models"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/mysql"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/store"
)

type TokenRegistryService interface {
	Add(req models.TokenAddRequest) (*models.TokenInfo, error)
	Get(tokenCode string) (*models.TokenInfo, error)
	Delete(tokenCode string) error
	List() ([]models.TokenInfo, error)
	FindToken(tokenCode string) (*models.TokenInfo, error)
	FindTokenByAddress(address string) (*models.TokenInfo, error)
}

type tokenRegistryService struct {
	mu           sync.RWMutex
	tokens       []models.TokenInfo
	tokensByCode map[string]models.TokenInfo
	tokensByAddr map[string]models.TokenInfo
}

func NewTokenRegistryService() TokenRegistryService {
	svc := &tokenRegistryService{
		tokensByCode: map[string]models.TokenInfo{},
		tokensByAddr: map[string]models.TokenInfo{},
	}
	svc.reloadCache()
	return svc
}

func (s *tokenRegistryService) Add(req models.TokenAddRequest) (*models.TokenInfo, error) {
	req.TokenCode = strings.TrimSpace(req.TokenCode)
	req.TokenAddress = strings.TrimSpace(req.TokenAddress)
	if req.TokenCode == "" {
		return nil, errors.New("tokenCode is required")
	}
	if req.TokenAddress == "" {
		return nil, errors.New("tokenAddress is required")
	}
	if req.Decimals < 0 {
		return nil, errors.New("decimals must be greater than or equal to 0")
	}

	row := store.TokenRegistryPO{
		Code:         req.TokenCode,
		NetworkCode:  configuredNetworkCode(),
		TokenAddress: req.TokenAddress,
		Decimals:     req.Decimals,
	}
	if err := mysql.DB().Where("code = ? and network_code = ?", row.Code, row.NetworkCode).Assign(row).FirstOrCreate(&row).Error; err != nil {
		return nil, err
	}
	s.reloadCache()
	return convertTokenRegistryPO(row), nil
}

func (s *tokenRegistryService) Get(tokenCode string) (*models.TokenInfo, error) {
	return s.FindToken(tokenCode)
}

func (s *tokenRegistryService) Delete(tokenCode string) error {
	tokenCode = strings.TrimSpace(tokenCode)
	if tokenCode == "" {
		return errors.New("tokenCode is required")
	}
	result := mysql.DB().Where("code = ? and network_code = ?", tokenCode, configuredNetworkCode()).Delete(&store.TokenRegistryPO{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("token not found")
	}
	s.reloadCache()
	return nil
}

func (s *tokenRegistryService) List() ([]models.TokenInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]models.TokenInfo, len(s.tokens))
	copy(result, s.tokens)
	return result, nil
}

func (s *tokenRegistryService) FindToken(tokenCode string) (*models.TokenInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row, ok := s.tokensByCode[strings.ToLower(strings.TrimSpace(tokenCode))]
	if !ok {
		return nil, errors.New("token not found")
	}
	token := row
	return &token, nil
}

func (s *tokenRegistryService) FindTokenByAddress(address string) (*models.TokenInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row, ok := s.tokensByAddr[strings.ToLower(strings.TrimSpace(address))]
	if !ok {
		return nil, errors.New("token not found")
	}
	token := row
	return &token, nil
}

func (s *tokenRegistryService) reloadCache() {
	var rows []store.TokenRegistryPO
	if err := mysql.DB().Where("network_code = ?", configuredNetworkCode()).Order("code asc").Find(&rows).Error; err != nil {
		return
	}
	tokens := make([]models.TokenInfo, 0, len(rows))
	byCode := make(map[string]models.TokenInfo, len(rows))
	byAddr := make(map[string]models.TokenInfo, len(rows))
	for _, row := range rows {
		token := *convertTokenRegistryPO(row)
		tokens = append(tokens, token)
		byCode[strings.ToLower(token.TokenCode)] = token
		byAddr[strings.ToLower(token.TokenAddress)] = token
	}
	s.mu.Lock()
	s.tokens = tokens
	s.tokensByCode = byCode
	s.tokensByAddr = byAddr
	s.mu.Unlock()
}

func convertTokenRegistryPO(row store.TokenRegistryPO) *models.TokenInfo {
	return &models.TokenInfo{
		TokenCode:    row.Code,
		NetworkCode:  row.NetworkCode,
		TokenAddress: row.TokenAddress,
		Decimals:     row.Decimals,
		CreatedAt:    models.TimeToMillis(row.CreatedAt),
		UpdatedAt:    models.TimeToMillis(row.UpdatedAt),
	}
}
