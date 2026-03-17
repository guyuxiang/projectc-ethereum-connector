package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/guyuxiang/projectc-ethereum-connector/pkg/config"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/models"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/mysql"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/store"
)

type ContractRegistryService interface {
	ListContracts() []models.ContractInfo
	ListWeb3Contracts() []models.Web3ContractInfo
	Push(message models.ContractConfigPushMessage)
	ApplyPush(pushRecordCode string) error
	PagePushRecords(request models.PageRequest[models.ContractConfigPushRecordQuery]) models.PageResponse[models.ContractConfigPushRecordDTO]
	FindContract(contractCode string) (*models.ContractInfo, error)
}

type contractRegistryService struct {
	networks map[string]config.NetworkConfig
}

func NewContractRegistryService() ContractRegistryService {
	svc := &contractRegistryService{
		networks: map[string]config.NetworkConfig{},
	}

	cfg := config.GetConfig()
	if cfg.Ethereum != nil {
		svc.networks[cfg.Ethereum.Network.Code] = cfg.Ethereum.Network
	}

	return svc
}

func (s *contractRegistryService) ListContracts() []models.ContractInfo {
	return s.listContractsFromDB()
}

func (s *contractRegistryService) ListWeb3Contracts() []models.Web3ContractInfo {
	return s.listWeb3ContractsFromDB()
}

func (s *contractRegistryService) Push(message models.ContractConfigPushMessage) {
	s.pushToDB(message)
}

func (s *contractRegistryService) ApplyPush(pushRecordCode string) error {
	return s.applyPushToDB(pushRecordCode)
}

func (s *contractRegistryService) PagePushRecords(request models.PageRequest[models.ContractConfigPushRecordQuery]) models.PageResponse[models.ContractConfigPushRecordDTO] {
	return s.pagePushRecordsFromDB(request)
}

func (s *contractRegistryService) FindContract(contractCode string) (*models.ContractInfo, error) {
	return s.findContractFromDB(contractCode)
}

func (s *contractRegistryService) listContractsFromDB() []models.ContractInfo {
	var records []store.CurrentContractConfigPO
	query := mysql.DB().Where("network_code = ?", configuredNetworkCode()).Order("network_code asc, contract_code asc")
	if err := query.Find(&records).Error; err != nil {
		return nil
	}
	result := make([]models.ContractInfo, 0, len(records))
	for _, record := range records {
		result = append(result, models.ContractInfo{
			Code:                record.ContractCode,
			NetworkCode:         record.NetworkCode,
			Address:             record.ContractAddress,
			InterfaceDefinition: record.ContractABI,
		})
	}
	return result
}

func (s *contractRegistryService) listWeb3ContractsFromDB() []models.Web3ContractInfo {
	contracts := s.listContractsFromDB()
	result := make([]models.Web3ContractInfo, 0, len(contracts))
	for _, contract := range contracts {
		network := s.networks[contract.NetworkCode]
		result = append(result, models.Web3ContractInfo{
			Contract: models.Web3Contract{
				Code:        contract.Code,
				NetworkCode: contract.NetworkCode,
				Address:     contract.Address,
				ABI:         contract.InterfaceDefinition,
			},
			Network: models.Web3Network{
				Code:        network.Code,
				NodeAddress: network.RPCURL,
				ChainID:     network.ChainID,
			},
		})
	}
	return result
}

func (s *contractRegistryService) pushToDB(message models.ContractConfigPushMessage) {
	items, _ := json.Marshal(message.PushItems)
	record := store.ContractConfigPushRecordPO{
		Code:        message.PushID,
		Description: message.Description,
		PushItems:   string(items),
	}
	mysql.DB().Where("code = ?", message.PushID).Assign(record).FirstOrCreate(&record)
}

func (s *contractRegistryService) applyPushToDB(pushRecordCode string) error {
	var record store.ContractConfigPushRecordPO
	if err := mysql.DB().Where("code = ?", pushRecordCode).First(&record).Error; err != nil {
		return errors.New("push record not found")
	}

	var items []models.ContractConfigPushItem
	if err := json.Unmarshal([]byte(record.PushItems), &items); err != nil {
		return err
	}

	for _, item := range items {
		item.NetworkCode = configuredNetworkCode()
		code := item.ContractCode + "_" + item.NetworkCode
		var current store.CurrentContractConfigPO
		err := mysql.DB().Where("code = ?", code).First(&current).Error
		if err != nil {
			history, _ := json.Marshal([]map[string]interface{}{
				{"pushRecordCode": pushRecordCode, "applyTime": nowMillis()},
			})
			current = store.CurrentContractConfigPO{
				Code:                    code,
				NetworkCode:             item.NetworkCode,
				ContractCode:            item.ContractCode,
				ContractAddress:         item.ContractAddress,
				ContractABI:             item.ContractABI,
				ContractDeployTxBlockNo: item.ContractDeployTxBlock,
				ApplyHistory:            string(history),
			}
			if createErr := mysql.DB().Create(&current).Error; createErr != nil {
				return createErr
			}
			registerContractAddressSubscription(item.NetworkCode, item.ContractAddress, item.ContractDeployTxBlock)
			continue
		}

		history := appendHistory(current.ApplyHistory, pushRecordCode)
		current.NetworkCode = item.NetworkCode
		current.ContractAddress = item.ContractAddress
		if item.ContractABI != "" {
			current.ContractABI = item.ContractABI
		}
		if item.ContractDeployTxBlock != 0 {
			current.ContractDeployTxBlockNo = item.ContractDeployTxBlock
		}
		current.ApplyHistory = history
		if saveErr := mysql.DB().Save(&current).Error; saveErr != nil {
			return saveErr
		}
		registerContractAddressSubscription(item.NetworkCode, item.ContractAddress, current.ContractDeployTxBlockNo)
	}

	return nil
}

func registerContractAddressSubscription(networkCode, address string, startBlock uint64) {
	if address == "" {
		return
	}
	code := fmt.Sprintf("%s_%s", networkCode, address)
	row := store.AddressSubscriptionPO{
		Code:             code,
		Address:          address,
		NetworkCode:      networkCode,
		StartBlockNumber: startBlock,
		EndBlockNumber:   ^uint64(0),
		NextBlockNumber:  startBlock,
		Status:           "ACTIVE",
	}
	mysql.DB().Where("code = ?", code).Assign(map[string]interface{}{
		"address":            address,
		"network_code":       networkCode,
		"start_block_number": startBlock,
		"status":             "ACTIVE",
	}).FirstOrCreate(&row)
}

func (s *contractRegistryService) pagePushRecordsFromDB(request models.PageRequest[models.ContractConfigPushRecordQuery]) models.PageResponse[models.ContractConfigPushRecordDTO] {
	pageNo := request.PageNo
	if pageNo <= 0 {
		pageNo = 1
	}
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	query := mysql.DB().Model(&store.ContractConfigPushRecordPO{})
	if request.Filter.CodeContains != "" {
		query = query.Where("code like ?", "%"+request.Filter.CodeContains+"%")
	}
	if request.Filter.DescriptionContains != "" {
		query = query.Where("description like ?", "%"+request.Filter.DescriptionContains+"%")
	}

	var total int64
	_ = query.Count(&total).Error

	var rows []store.ContractConfigPushRecordPO
	_ = query.Order("id desc").Offset((pageNo - 1) * pageSize).Limit(pageSize).Find(&rows).Error

	records := make([]models.ContractConfigPushRecordDTO, 0, len(rows))
	for _, row := range rows {
		records = append(records, models.ContractConfigPushRecordDTO{
			Code:        row.Code,
			Description: row.Description,
		})
	}

	return models.PageResponse[models.ContractConfigPushRecordDTO]{
		PageNo:     pageNo,
		PageSize:   pageSize,
		TotalCount: int(total),
		Records:    records,
	}
}

func (s *contractRegistryService) findContractFromDB(contractCode string) (*models.ContractInfo, error) {
	var row store.CurrentContractConfigPO
	if err := mysql.DB().Where("network_code = ? and contract_code = ?", configuredNetworkCode(), contractCode).First(&row).Error; err != nil {
		return nil, errors.New("contract not found")
	}
	return &models.ContractInfo{
		Code:                row.ContractCode,
		NetworkCode:         row.NetworkCode,
		Address:             row.ContractAddress,
		InterfaceDefinition: row.ContractABI,
	}, nil
}

func appendHistory(raw string, pushRecordCode string) string {
	type historyItem struct {
		PushRecordCode string `json:"pushRecordCode"`
		ApplyTime      int64  `json:"applyTime"`
	}
	var history []historyItem
	_ = json.Unmarshal([]byte(raw), &history)
	history = append(history, historyItem{PushRecordCode: pushRecordCode, ApplyTime: nowMillis()})
	if len(history) > 100 {
		history = history[len(history)-100:]
	}
	out, _ := json.Marshal(history)
	return string(out)
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
}
