# projectc-ethereum-connector

`projectc-ethereum-connector` 是一个基于 Go 实现的以太坊连接服务，用于承接 `projectc-chain-connector-java` 中 EVM Connector 的核心能力，并以当前仓库的 Gin + Service + Store 框架组织方式提供 HTTP 接口、链上读写、合约配置管理、上链状态推进和订阅回调能力。

当前版本已经不是初始化模板，而是一个可运行的业务服务。

## 1. 服务定位

本服务面向 ProjectC 体系内的上层业务，主要提供以下能力：

- 通用 EVM JSON-RPC 读写能力
- 钱包原生转账
- SC+ / Bridge 相关合约交易签名与提交
- 合约配置推送、应用、查询
- 交易订阅与地址订阅
- 订阅回调、取消回调、确认复查
- 上链记录持久化、状态推进、重签重提

服务目标不是做通用钱包节点代理，而是做“ProjectC 面向业务语义的以太坊连接层”。

## 2. 当前实现范围

### 2.1 已实现能力

- 基础通用接口
  - `tx-send`
  - `tx-query`
  - `address-balance`
  - `latest-block`
  - `token-supply`
  - `token-balance`
- 钱包能力
  - `wallet/faucet`
- SC+ / Bridge 能力
  - `dtt-send-settle`
  - `auto-reject`
  - `instant-on-ramp`
  - `issue`
  - `finance`
  - `issueF`
  - 对应 query 接口
- 合约配置能力
  - 合约列表
  - 合约配置 push
  - 合约配置 apply
  - web3 contract info
- 订阅能力
  - `tx-subscribe`
  - `tx-subscribe-cancel`
  - `address-subscribe`
  - `address-subscribe-cancel`
- 上链状态管理
  - `INIT -> PROCESSING -> SUCCESS / FAILED`
  - nonce 冲突识别
  - `TO_BE_RESIGN`
  - 重签重提
- 事件解码
  - `ChainEventType` 业务枚举映射
  - ABI 解码
  - Java 风格事件数据结构转换

### 2.2 当前仍是简化实现的部分

- 订阅运行模型仍是单进程定时轮询，不是 Java 中多 consumer + 多 exchange + job 的完全同构实现
- 当前没有系统化集成测试
- Swagger 说明未完整反映所有迁移后的业务语义

## 3. 总体架构

项目沿用当前仓库的分层方式：

```bash
projectc-ethereum-connector/
├── cmd/                       # 程序启动入口
├── docs/                      # swagger 文档
├── etc/                       # 配置文件
├── pkg/
│   ├── config/                # 配置模型与读取
│   ├── controller/            # HTTP Controller
│   ├── log/                   # 日志
│   ├── middleware/            # 鉴权中间件
│   ├── models/                # DTO / 响应模型
│   ├── mysql/                 # MySQL 初始化
│   ├── rabbitmq/              # RabbitMQ 初始化与发布
│   ├── route/                 # Gin 路由注册
│   ├── service/               # 业务逻辑
│   ├── store/                 # GORM 表模型与迁移
│   └── util/                  # 通用工具
└── hack/                      # 启动与部署脚本
```

### 3.1 分层职责

- `controller`
  - 负责 HTTP 参数绑定、错误返回、调用 service
  - 不做复杂业务决策
- `service`
  - 负责业务语义、链上交互、状态机推进、事件解码、订阅推进
- `store`
  - 定义持久化表结构并负责迁移
- `rabbitmq`
  - 负责 MQ 连接与发布
- `config`
  - 定义服务配置模型

### 3.2 运行模型

当前服务是单体进程内运行，核心后台行为由 [connector.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/controller/connector.go#L33) 启动的 ticker 驱动：

- 每 15 秒执行一次
  - `onchain.Refresh()`
  - `subscriptions.Refresh()`

这意味着当前系统是“数据库状态驱动 + 后台轮询推进”模型，而不是“多 consumer 分层异步编排”模型。

## 4. 核心模块说明

### 4.1 启动入口

[main.go](/usr/src/golang/workspace/projectc-ethereum-connector/cmd/main.go)

启动逻辑：

1. 初始化 MySQL
2. 自动迁移 connector 相关表
3. 初始化 RabbitMQ
4. 创建 Gin 路由
5. 启动 HTTP 服务

MySQL 与 RabbitMQ 都支持“未配置则跳过”。

### 4.2 路由层

[routes.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/route/routes.go)

主要接口分组：

- `/inner/chain-invoke/:networkCode/common/*`
- `/inner/chain-data/:networkCode/common/*`
- `/inner/chain-data-subscribe/:networkCode/*`
- `/inner/chain-invoke/:networkCode/wallet/*`
- `/inner/chain-invoke/:networkCode/scplus/*`
- `/inner/contract/*`
- `/open/*`

除 `/ping`、`/version`、`/swagger` 以外，其余接口默认走 Basic Auth。

### 4.3 EVM 通用能力

[ethereum_rpc.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/ethereum_rpc.go)

负责：

- 直接调用节点 JSON-RPC
- 查询交易、区块、余额、代币余额、总量
- 对 receipt logs 进行 ABI 解码
- 将原始事件映射为 Java 风格 `ChainEventType`
- 将事件 data 转成 ProjectC 使用的业务 JSON 结构

当前 `tx-query` 返回的不只是原始链上结果，而是带业务语义的：

- `tx`
- `txEvents`

### 4.4 钱包与合约交易签名

相关文件：

- [wallet.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/wallet.go)
- [contract_tx.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/contract_tx.go)
- [scplus.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/scplus.go)
- [signer_nonce.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/signer_nonce.go)

职责：

- EIP-1559 动态费交易构造
- nonce 管理
- 原生转账签名
- 合约调用 ABI 编码
- SC+ 业务参数组装
- issue / finance / issueF 等复杂 tuple 结构编码

`SignerNonceService` 会维护本地签名地址在不同网络上的 nonce，并支持重签时强制重置“下一次分配的 nonce”。

### 4.5 OnchainRecord 状态机

[onchain_record.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/onchain_record.go)

这是写链闭环的核心。

主要职责：

- 持久化 `OnchainRecord`
- 提交签名交易
- 定时刷新链上状态
- 识别已知交易错误
- 识别 nonce 冲突
- 切换到 `TO_BE_RESIGN`
- 从原始请求数据重建 prepared tx
- 重新签名并重新提交
- 自动为已提交交易登记 `tx-subscribe`

当前主要状态：

- `INIT`
- `PROCESSING`
- `TO_BE_RESIGN`
- `SUCCESS`
- `FAILED`

### 4.6 合约配置管理

[contract_registry.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/contract_registry.go)

主要职责：

- 管理 push record
- 管理 current contract config
- 提供 contract list / web3 contract info
- 支持 push -> apply
- 应用配置后自动登记对应合约地址的 `address-subscribe`

这使得合约地址的事件扫描可以在配置应用后自动开始。

### 4.7 订阅系统

[subscription.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/subscription.go)

这是当前服务最复杂的运行时模块之一。

#### `tx-subscribe`

流程：

1. 记录 `TxSubscriptionPO`
2. 后台轮询交易是否上链
3. 上链后生成 `TxCallbackRecordPO`
4. 发布 tx callback
5. 失败时按退避时间重试
6. 已回调成功后等待确认块数
7. 达到确认块数后复查：
   - 如果交易消失，发取消回调
   - 如果 payload 改变，重发回调
   - 如果稳定，结束 waiting-check

#### `address-subscribe`

流程：

1. 记录 `AddressSubscriptionPO`
2. 后台按块区间扫描
3. 命中地址相关交易后生成 `tx-subscribe`
4. 同时记录 `AddressSyncWaitingCheckPO`
5. 确认块数后重新扫描同一区间
6. 如果确认期内出现新的地址相关交易，再补成 `tx-subscribe`

当前地址命中规则包括：

- `from == target`
- `to == target`
- log address == target
- ERC20 `Transfer` 中 `from` / `to` == target

#### MQ 回调

当前支持两类对外消息：

- 普通回调
- 取消回调

配置项：

- `rabbitmq.txCallbackExchange`
- `rabbitmq.txCallbackExchangeType`
- `rabbitmq.txCallbackCancelExchange`
- `rabbitmq.txCallbackCancelExchangeType`

如果未配置，会回退到默认 `exchange + exchangeType + routingKey`。

## 5. 数据表说明

[connector_store.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/store/connector_store.go)

当前自动迁移的核心表：

- `ContractConfigPushRecordPO`
- `CurrentContractConfigPO`
- `OnchainRecordPO`
- `SignerNoncePO`
- `TxSubscriptionPO`
- `AddressSubscriptionPO`
- `AddressSyncWaitingCheckPO`
- `TxCallbackRecordPO`

### 5.1 关键表职责

- `OnchainRecordPO`
  - 记录业务请求对应的链上提交记录
  - 保存签名交易、raw tx、nonce、链上状态、错误信息
- `TxSubscriptionPO`
  - 记录待监听的交易
- `TxCallbackRecordPO`
  - 记录回调 payload、回调状态、确认检查状态、重试信息
- `AddressSubscriptionPO`
  - 记录地址扫描订阅
- `AddressSyncWaitingCheckPO`
  - 记录地址扫描区间的确认复查任务

## 6. 主要业务流程

### 6.1 钱包 / SC+ 写链流程

1. controller 接收请求
2. service 组装交易参数
3. 生成 prepared onchain record
4. 落库
5. 提交交易
6. 自动登记 `tx-subscribe`
7. 后台刷新 receipt
8. 后台触发回调
9. 确认块数后复查稳定性

### 6.2 合约配置生效流程

1. push contract config record
2. apply push record
3. 更新 current contract config
4. 自动登记合约地址 `address-subscribe`
5. 后续地址扫描命中交易
6. 下沉为 `tx-subscribe`
7. 回调 tx 与 txEvents

## 7. 配置说明

默认配置文件：  
[projectc-ethereum-connector.yaml](/usr/src/golang/workspace/projectc-ethereum-connector/etc/projectc-ethereum-connector.yaml)

### 7.1 基础配置

- `server.host`
- `server.port`
- `gin.mode`
- `auth.username`
- `auth.password`

### 7.2 MySQL

- `mysql.dsn`
- `mysql.maxIdleConns`
- `mysql.maxOpenConns`

如果 `dsn` 为空，则 MySQL 初始化跳过，部分状态会退化为内存行为。

### 7.3 RabbitMQ

- `rabbitmq.url`
- `rabbitmq.exchange`
- `rabbitmq.exchangeType`
- `rabbitmq.queue`
- `rabbitmq.routingKey`
- `rabbitmq.txCallbackExchange`
- `rabbitmq.txCallbackExchangeType`
- `rabbitmq.txCallbackCancelExchange`
- `rabbitmq.txCallbackCancelExchangeType`

如果 `url` 为空，则 RabbitMQ 初始化跳过，回调发布会不可用。

### 7.4 Ethereum

- `ethereum.networks`
  - `code`
  - `rpcUrl`
  - `chainId`
  - `blockchainExplorerUrl`
- `ethereum.contracts`
  - `code`
  - `networkCode`
  - `address`
  - `abi`
  - `deployBlockNumber`

### 7.5 Connector

- `connector.wallets`
  - 钱包私钥配置
- `connector.onchains`
  - 不同 `networkCode + onchainType` 对应的签名人和目标合约配置

## 8. 运行方式

### 8.1 本地运行

```bash
cd /usr/src/golang/workspace/projectc-ethereum-connector
go build ./...
bash hack/start.sh
```

也可以直接：

```bash
go run ./cmd
```

### 8.2 依赖说明

建议在完整模式下提供：

- 一个可访问的 EVM 节点
- MySQL
- RabbitMQ

如果只做接口调试，可在无 MySQL / 无 RabbitMQ 模式下启动，但订阅和持久化语义会退化。

## 9. 当前设计特点

### 9.1 当前优点

- 迁移成本低，能较快承接 Java 的业务能力
- 所有关键逻辑都集中在 Go 服务内部，排障路径短
- 对外接口和业务语义已基本贴近 Java 版本

### 9.2 当前架构限制

- 运行模型是单进程轮询，而不是 Java 的多 consumer + 多 exchange + job 分层
- service 直接操作 GORM，repository 层尚未抽出
- 业务落库与 MQ 发布尚未使用 outbox 模式
- 还缺少系统化测试

## 10. 已知边界

当前版本已经可以认为“核心功能逻辑迁移完成”，但仍有以下边界：

- 运行时架构未 1:1 复制 Java 异步编排模型
- 没有完整集成测试与回归测试
- 某些事件结构虽然已对齐业务字段，但仍可能与 Java 极个别 decoder 存在细节差异

## 11. 后续建议

如果继续演进，建议优先做以下事情：

1. 补集成测试
2. 补状态机回归测试
3. 抽 repository 层
4. 引入 outbox 模式
5. 将后台 ticker 推进逐步拆为独立 worker / consumer

## 12. 相关文件索引

- 启动入口：[main.go](/usr/src/golang/workspace/projectc-ethereum-connector/cmd/main.go)
- 路由注册：[routes.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/route/routes.go)
- 控制器入口：[connector.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/controller/connector.go)
- EVM 通用能力：[ethereum_rpc.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/ethereum_rpc.go)
- 钱包签名：[wallet.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/wallet.go)
- 合约签名：[contract_tx.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/contract_tx.go)
- SC+ 业务：[scplus.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/scplus.go)
- 上链状态机：[onchain_record.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/onchain_record.go)
- 订阅系统：[subscription.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/subscription.go)
- 合约配置管理：[contract_registry.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/service/contract_registry.go)
- 存储模型：[connector_models.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/store/connector_models.go)
- 自动迁移：[connector_store.go](/usr/src/golang/workspace/projectc-ethereum-connector/pkg/store/connector_store.go)
