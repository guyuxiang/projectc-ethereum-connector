package service

import (
	"errors"
	"strings"

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

type tokenRegistryService struct{}

func NewTokenRegistryService() TokenRegistryService {
	return &tokenRegistryService{}
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
	return nil
}

func (s *tokenRegistryService) List() ([]models.TokenInfo, error) {
	var rows []store.TokenRegistryPO
	if err := mysql.DB().Where("network_code = ?", configuredNetworkCode()).Order("code asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]models.TokenInfo, 0, len(rows))
	for _, row := range rows {
		result = append(result, *convertTokenRegistryPO(row))
	}
	return result, nil
}

func (s *tokenRegistryService) FindToken(tokenCode string) (*models.TokenInfo, error) {
	var row store.TokenRegistryPO
	if err := mysql.DB().Where("code = ? and network_code = ?", strings.TrimSpace(tokenCode), configuredNetworkCode()).First(&row).Error; err != nil {
		return nil, errors.New("token not found")
	}
	return convertTokenRegistryPO(row), nil
}

func (s *tokenRegistryService) FindTokenByAddress(address string) (*models.TokenInfo, error) {
	var row store.TokenRegistryPO
	if err := mysql.DB().Where("lower(token_address) = ? and network_code = ?", strings.ToLower(strings.TrimSpace(address)), configuredNetworkCode()).First(&row).Error; err != nil {
		return nil, errors.New("token not found")
	}
	return convertTokenRegistryPO(row), nil
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
