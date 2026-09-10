# 项目总览

[English](./PROJECT-OVERVIEW.md)

一页纸看懂 GinBlade：每个部分是什么、彼此怎么拼装、某项能力该去哪儿找、以及哪些决定是刻意的而非偶然。建议先读本文，再读 [ARCHITECTURE.zh-CN.md](./ARCHITECTURE.zh-CN.md)（设计理由）、[事务](docs/zh/transactions.md)（怎么让多行写入变原子）与[快速上手](docs/zh/quickstart.md)（可直接抄的代码）。

## 这是什么

`github.com/arixbit/ginblade` 是一个**有观点、可直接运行**的 Go 后端骨架，面向需要清晰边界、可独立部署多进程、且能从本地开发一路验证到 CI 的服务。

它**不是**产品，也不是通用架构。业务模块刻意保持最小——它们存在的意义是演示各层如何拼装。

## 一眼全貌

| | |
|---|---|
| Module | `github.com/arixbit/ginblade` |
| 语言 | Go 1.26（toolchain go1.26.5） |
| 包数 | 22 |
| Go 文件 | 76（其中 30 个是测试） |
| Go 代码行数 | 约 6.7k |

### 技术栈

| 关注点 | 选型 |
|---|---|
| HTTP | Gin v1.10 |
| ORM / 数据库 | GORM v1.30 + PostgreSQL（pgx v5） |
| 异步任务 | Asynq v0.26（基于 Redis） |
| 缓存 | go-redis v9 |
| 日志 | zap（JSON / console，携带 `trace_id`） |
| 鉴权 | golang-jwt/v5（HS256） |
| 参数校验 | go-playground/validator v10 |
| 配置 | 环境变量（godotenv 加载 .env） |
| 容器 | 多阶段 Dockerfile，非 root `app` 用户 |
| 编排 | Docker Compose（Postgres 17、Redis 7、migrate、api、worker） |

## 进程模型

三个独立二进制，启动序列一致：
`config.LoadEnv → config.Load → bootstrap.InitRuntime → bootstrap.Init<进程>
→ app.New<Server|Worker> → 运行 → 基于信号优雅停机`。

| 进程 | 必需 | 可选 | 角色 |
|---|---|---|---|
| `cmd/api` | Postgres | Redis、JWT | HTTP 服务（Gin） |
| `cmd/worker` | Redis | Postgres | Asynq 任务消费者 |
| `cmd/migrate` | Postgres | — | GORM `AutoMigrate` 后退出 |

## 分层

```
HTTP 壳层（耦合 Gin）
  handler      绑定/校验请求、调用 service、写响应
  middleware   链路追踪、恢复、超时、CORS、认证、限流
  router       路由注册（未配置的可选模块自动跳过）
───────────────────────────────────────────────────────────────
应用核心（框架无关）
  service      业务规则；声明它消费的接口
  repository   持久化（GORM）；context 级事务
  model        GORM 表模型
  task         异步任务类型与载荷
  worker       Asynq 服务端、处理器、trace 中间件
  taskqueue    队列边界封装
  errcode      业务错误码
───────────────────────────────────────────────────────────────
基础设施（pkg/，可复用）
  auth · cache · database · log · response · validator
```

代码实际遵守的规则：

- **`handler` 不直接接触数据库或队列**，只调用 service，并把错误映射进响应信封。
- **service 声明它消费的接口**——`ExampleRepository`、`ExampleQueue`、`WalletRepository`、`WalletCache`、`TransactionRunner`。具体实现由 `bootstrap` 注入，这正是 service 层无需数据库即可单元测试的原因。
- **`repository` 只认识 GORM。** 事务通过 context 传递：`InTx` 开启事务（或复用已有事务），`dbFromContext` 透明地使用当前事务。
- **`pkg/` 保持可复用**，其中任何代码都不 import `internal/`。

## 两条参考链路

骨架自带两条并存的链路。`Example` 是模块所需的最小形态；`Wallet` 是同一形态把两个更难的模式补齐后的样子。二者分开而非合并，是为了各自保持可读。

| 能力 | 链路 | 位置 |
|---|---|---|
| `handler → service → repository` 路径 | `Example` | `internal/service/example.go` |
| 消费方定义接口 + 注入 | `Example` | `internal/service/example.go` |
| 异步任务发布，`trace_id` 写入载荷 | `Example` | `internal/service/example.go`、`internal/task/example.go` |
| service 层编排的多行事务 | `Wallet` | `internal/service/wallet.go` |
| 随 context 传递的事务 | `Wallet` | `internal/repository/tx.go`、`internal/repository/wallet.go` |
| 仓储哨兵错误映射到 `errcode` | `Wallet` | `internal/service/wallet.go` |
| cache-aside 读 + 版本号失效 | `Wallet` | `internal/service/wallet.go` |

### 事务

契约分两半：

- **service 开启事务**：调用 `tx.InTx`，即 `service.TransactionRunner`。具体实现 `repository.TxRunner` 在组合根由 `*gorm.DB` 构建，并在 `internal/server.go` 注入，因此 service 永远不需要 import GORM。
- **repository 加入事务**：每个仓储方法都通过 `dbFromContext(ctx, r.db)` 取句柄——context 中有事务就返回事务，否则返回普通连接。

`WalletRepository.Debit` 把余额判断放进 SQL 的 `WHERE` 条件（`id = ? AND balance >= ?`），使"检查 + 更新"成为原子操作，因此两个并发转账不可能把同一个钱包扣穿。领域失败以哨兵错误形式抛出（`ErrInsufficientBalance`、`ErrWalletNotFound`），由 service 翻译成 `errcode`，从而不让 HTTP 语义渗进持久层。

### 缓存

`WalletService.List` 通过 service 自己声明的 `WalletCache` 接口做 cache-aside 读，该接口由 `internal/server.go` 从 `*cache.Client` 适配而来，Redis 未配置时返回 `nil`。失效采用版本号：写操作 bump `wallet:list:version`，每个列表 key 都嵌该版本号，因此一次 `Set` 即可让全部列表缓存失效，无需扫描或删除 key。

## HTTP 接口面

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/health` | 真实 HTTP 状态码；必需依赖不可用时 503 |
| POST | `/api/v1/auth/token` | 仅在配置 `JWT_SECRET` 后注册 |
| GET | `/api/v1/auth/me` | Bearer 认证；仅在配置 `JWT_SECRET` 后注册 |
| GET | `/api/v1/examples` | 分页 `?limit=&offset=` |
| POST | `/api/v1/examples` | `{"name":"..."}` |
| POST | `/api/v1/examples/tasks` | 发布异步任务；需要 Redis |
| GET | `/api/v1/wallets` | 配置 Redis 时走 cache-aside |
| POST | `/api/v1/wallets` | `{"name":"alice","balance":100}` |
| GET | `/api/v1/wallets/:id` | 不存在返回 `NOT_FOUND` |
| POST | `/api/v1/wallets/transfers` | `{"from_id":1,"to_id":2,"amount":50}` |

中间件链：`TraceLogger → Recovery → Timeout → CORS → RateLimit`（仅当 `RATE_LIMIT_PER_MINUTE > 0` 时启用限流）。

### 响应信封

```json
{ "code": 0, "msg": "success", "data": {...}, "metadata": { "trace_id": "..." } }
```

- 成功：`code = 0`，HTTP 200。
- 业务错误：`{ code, msg, reason, metadata }`，**按约定返回 HTTP 200**；信封里的 `code` 才是语义来源。`/health` 是刻意的例外，使用真实状态码。

| code | reason | 含义 |
|---|---|---|
| 1001 | `INVALID_PARAMS` | 请求参数校验失败 |
| 1002 | `UNAUTHORIZED` | 未认证 / 凭证无效 |
| 1003 | `PERMISSION_DENIED` | 已认证但无权限 |
| 1004 | `TOO_MANY_REQUESTS` | 触发限流 |
| 1005 | `REQUEST_TIMEOUT` | 请求超时 |
| 2001 | `NOT_FOUND` | 资源不存在 |
| 2002 | `INSUFFICIENT_BALANCE` | 转账余额不足 |
| 9001 | `INTERNAL_ERROR` | 未预期的服务端错误 |
| 9002 | `DATABASE_ERROR` | 数据库操作失败 |
| 9003 | `QUEUE_UNAVAILABLE` | 队列未配置 |
| 9004 | `QUEUE_ERROR` | 任务发布失败 |

## 异步任务

- **队列**：`critical:6 / default:3 / low:1`，并发 10。
- **重试**：`MaxRetry(5)`，指数退避 `5s × 2^n`，上限 1 小时；失败由 `ErrorHandler` 记录。
- **追踪**：发布侧把 `trace_id` 写入载荷，Worker 侧的 `TraceMiddleware` 恢复它（没有则生成任务域 trace），并记录任务 id、队列名、重试次数。

## 配置

全部配置来自环境变量，由 `config.Load()` 一次性读取并带默认值。`config.LoadEnv` 加载 `cmd/<进程>/.env`，回退到仓库根目录 `.env`。

`config/types.go` 把配置分为 `Server`、`Postgres`、`Redis`、`Auth`、`Cors`、`Log`、`RateLimit` 七组。**Redis 与 JWT 均为可选**：未配置时相关路由不注册、`/health` 上报 `not_configured`、缓存读降级为查库，其余功能照常工作。完整变量表见 `.env.example`。

## 工程化

| 目标 | 作用 |
|---|---|
| `make ci` | `fmt-check + tidy-check + verify + vet + test-race + build` |
| `make test` / `make test-race` | 单元测试，可选竞态检测 |
| `make integration-up` / `test-integration` / `integration-down` | 在隔离 Compose 项目中跑真实 Postgres + Redis 集成测试 |
| `make compose-up` / `compose-down` / `compose-logs` | 本地全套编排 |
| `make build` | 构建 `api`、`worker`、`migrate` 到 `bin/` |
| `make lint` | golangci-lint |

集成测试带 `integration` build tag，需要 `TEST_POSTGRES_DSN`、`TEST_REDIS_ADDR`、`TEST_REDIS_CACHE_DB`、`TEST_REDIS_QUEUE_DB`；Makefile 提供安全的本地默认值，`INTEGRATION_PROJECT` / `INTEGRATION_POSTGRES_PORT` / `INTEGRATION_REDIS_PORT` 让多工作区并行时互不冲突。

### 测试策略

三层，各证一件别层证不了的事：

| 层次 | 手段 | 证明什么 |
|---|---|---|
| Service | 手写 mock 仓储 / 缓存 / `TransactionRunner` | 业务规则、错误映射、调用顺序，无需数据库 |
| Repository | GORM **DryRun** 夹具（`internal/repository/example_test.go`）并捕获语句 | 生成的 SQL——包括钱包的 `balance >= ?` 防透支条件——无需数据库 |
| 集成 | 隔离 schema 的真实 Postgres + 真实 Redis（`tests/integration/`） | 真正的提交/回滚原子性、`ErrRecordNotFound` 翻译、缓存与队列行为 |

`tests/integration/wallet_test.go` 是第三层的范例：它断言一次转账会把两条腿加审计记录一起提交、强制失败会把三者一起回滚、以及透支会被拒绝且源钱包余额不变。

### CI

| Job | 作用 |
|---|---|
| `test` | 单元检查 + 针对服务容器的集成测试，覆盖率上传 Codecov |
| `lint` | golangci-lint v2（`--build-tags=integration`） |
| `vulncheck` | `govulncheck`；**只报告，不阻塞合并** |
| `container` | 校验 Compose、构建镜像、断言非 root 用户、启动全套栈，并冒烟测试 health → 创建 → 投递 → Worker 消费 |
| `release` | 打 `v*` tag 时由 GoReleaser 发布 linux/darwin × amd64/arm64 二进制 |

`.github/workflows/docs.yml` 以 strict 模式构建 mkdocs 站点并部署到 GitHub Pages。

### 文档构成

| 文件 | 用途 |
|---|---|
| `README.md` / `README.zh-CN.md` | 入门、配置、API 接口面 |
| `ARCHITECTURE.md` / `ARCHITECTURE.zh-CN.md` | 设计理由、分层规则、生命周期 |
| `I18N.md` / `I18N.zh-CN.md` | 如何在基于本骨架的服务里接入多语言 |
| `PROJECT-OVERVIEW.md` / `PROJECT-OVERVIEW.zh-CN.md` | 本文 |
| `docs/en/`、`docs/zh/` | mkdocs-material 站点（index、quickstart、transactions、architecture、i18n-guide） |

`docs/en/architecture.md` 与 `docs/zh/architecture.md` 是根目录 `ARCHITECTURE*.md` 的**副本**。改了一处就要同步另一处，否则线上站点会变旧。

## 值得知道的约定

- **业务错误返回 HTTP 200。** 这是刻意选择，便于网关统一处理；若团队偏好真实状态码，改 `pkg/response` 即可。
- **`vulncheck` 不阻塞合并。** 标准库漏洞要等工具链补丁，因此该 job 只报告不失败；第三方依赖漏洞仍应清零。
- **`handler` 以下的新代码与框架无关。** 只有 `internal/handler`、`internal/middleware`、`internal/router`、`internal/server.go` 与 `pkg/response` 引用 Gin。这正是替换 HTTP 壳层成本很低的原因。
- **新增模型必须注册**到 `cmd/migrate/main.go` 的 `AutoMigrate` 调用里。
