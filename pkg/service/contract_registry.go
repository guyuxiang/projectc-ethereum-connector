package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/guyuxiang/projectc-ethereum-connector/pkg/config"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/models"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/mysql"
	"github.com/guyuxiang/projectc-ethereum-connector/pkg/store"
)

type ContractRegistryService interface {
	ListContracts(networkCode string) []models.ContractInfo
	ListWeb3Contracts() []models.Web3ContractInfo
	Push(message models.ContractConfigPushMessage)
	ApplyPush(pushRecordCode string) error
	PagePushRecords(request models.PageRequest[models.ContractConfigPushRecordQuery]) models.PageResponse[models.ContractConfigPushRecordDTO]
	FindContract(networkCode, contractCode string) (*models.ContractInfo, error)
}

type pushRecord struct {
	models.ContractConfigPushMessage
	applied bool
}

type contractRegistryService struct {
	mu          sync.RWMutex
	current     map[string]models.ContractInfo
	networks    map[string]config.NetworkConfig
	pushRecords []pushRecord
}

func NewContractRegistryService() ContractRegistryService {
	svc := &contractRegistryService{
		current:  map[string]models.ContractInfo{},
		networks: map[string]config.NetworkConfig{},
	}

	cfg := config.GetConfig()
	if cfg.Ethereum != nil {
		for _, network := range cfg.Ethereum.Networks {
			svc.networks[network.Code] = network
		}
		for _, contract := range cfg.Ethereum.Contracts {
			key := contract.NetworkCode + ":" + contract.Code
			svc.current[key] = models.ContractInfo{
				Code:                contract.Code,
				NetworkCode:         contract.NetworkCode,
				Address:             contract.Address,
				InterfaceDefinition: contract.ABI,
			}
		}
	}

	return svc
}

func (s *contractRegistryService) ListContracts(networkCode string) []models.ContractInfo {
	if mysql.DB() != nil {
		return s.listContractsFromDB(networkCode)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []models.ContractInfo
	for _, contract := range s.current {
		if networkCode == "" || contract.NetworkCode == networkCode {
			result = append(result, contract)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].NetworkCode == result[j].NetworkCode {
			return result[i].Code < result[j].Code
		}
		return result[i].NetworkCode < result[j].NetworkCode
	})
	return result
}

func (s *contractRegistryService) ListWeb3Contracts() []models.Web3ContractInfo {
	if mysql.DB() != nil {
		return s.listWeb3ContractsFromDB()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []models.Web3ContractInfo
	for _, contract := range s.current {
		network := s.networks[contract.NetworkCode]
		result = append(result, models.Web3ContractInfo{
			Contract: models.Web3Contract{
				Code:        contract.Code,
				NetworkCode: contract.NetworkCode,
				Address:     contract.Address,
				ABI:         contract.InterfaceDefinition,
			},
			Network: models.Web3Network{
				Code:                  network.Code,
				NodeAddress:           network.RPCURL,
				ChainID:               network.ChainID,
				BlockchainExplorerURL: network.BlockchainExplorerURL,
			},
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Contract.NetworkCode == result[j].Contract.NetworkCode {
			return result[i].Contract.Code < result[j].Contract.Code
		}
		return result[i].Contract.NetworkCode < result[j].Contract.NetworkCode
	})
	return result
}

func (s *contractRegistryService) Push(message models.ContractConfigPushMessage) {
	if mysql.DB() != nil {
		s.pushToDB(message)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pushRecords = append(s.pushRecords, pushRecord{ContractConfigPushMessage: message})
}

func (s *contractRegistryService) ApplyPush(pushRecordCode string) error {
	if mysql.DB() != nil {
		return s.applyPushToDB(pushRecordCode)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for idx := range s.pushRecords {
		if s.pushRecords[idx].PushID != pushRecordCode {
			continue
		}
		for _, item := range s.pushRecords[idx].PushItems {
			s.current[item.NetworkCode+":"+item.ContractCode] = models.ContractInfo{
				Code:                item.ContractCode,
				NetworkCode:         item.NetworkCode,
				Address:             item.ContractAddress,
				InterfaceDefinition: item.ContractABI,
			}
		}
		s.pushRecords[idx].applied = true
		return nil
	}
	return errors.New("push record not found")
}

func (s *contractRegistryService) PagePushRecords(request models.PageRequest[models.ContractConfigPushRecordQuery]) models.PageResponse[models.ContractConfigPushRecordDTO] {
	if mysql.DB() != nil {
		return s.pagePushRecordsFromDB(request)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	filtered := make([]models.ContractConfigPushRecordDTO, 0, len(s.pushRecords))
	codeContains := strings.ToLower(request.Filter.CodeContains)
	descContains := strings.ToLower(request.Filter.DescriptionContains)
	for _, record := range s.pushRecords {
		if codeContains != "" && !strings.Contains(strings.ToLower(record.PushID), codeContains) {
			continue
		}
		if descContains != "" && !strings.Contains(strings.ToLower(record.Description), descContains) {
			continue
		}
		filtered = append(filtered, models.ContractConfigPushRecordDTO{
			Code:        record.PushID,
			Description: record.Description,
		})
	}

	pageNo := request.PageNo
	if pageNo <= 0 {
		pageNo = 1
	}
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	start := (pageNo - 1) * pageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	return models.PageResponse[models.ContractConfigPushRecordDTO]{
		PageNo:     pageNo,
		PageSize:   pageSize,
		TotalCount: len(filtered),
		Records:    filtered[start:end],
	}
}

func (s *contractRegistryService) FindContract(networkCode, contractCode string) (*models.ContractInfo, error) {
	if mysql.DB() != nil {
		return s.findContractFromDB(networkCode, contractCode)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	contract, ok := s.current[networkCode+":"+contractCode]
	if !ok {
		return nil, errors.New("contract not found")
	}
	copy := contract
	return &copy, nil
}

func (s *contractRegistryService) listContractsFromDB(networkCode string) []models.ContractInfo {
	var records []store.CurrentContractConfigPO
	query := mysql.DB().Order("network_code asc, contract_code asc")
	if networkCode != "" {
		query = query.Where("network_code = ?", networkCode)
	}
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
	contracts := s.listContractsFromDB("")
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
				Code:                  network.Code,
				NodeAddress:           network.RPCURL,
				ChainID:               network.ChainID,
				BlockchainExplorerURL: network.BlockchainExplorerURL,
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
	if mysql.DB() == nil || address == "" {
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

func (s *contractRegistryService) findContractFromDB(networkCode, contractCode string) (*models.ContractInfo, error) {
	var row store.CurrentContractConfigPO
	if err := mysql.DB().Where("network_code = ? and contract_code = ?", networkCode, contractCode).First(&row).Error; err != nil {
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
