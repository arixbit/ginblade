# GinBlade · 中文使用介绍

[![CI](https://github.com/arixbit/ginblade/actions/workflows/ci.yml/badge.svg)](https://github.com/arixbit/ginblade/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/arixbit/ginblade?label=Go)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/arixbit/ginblade)](https://goreportcard.com/report/github.com/arixbit/ginblade)
[![Version](https://img.shields.io/github/v/tag/arixbit/ginblade?label=version)](https://github.com/arixbit/ginblade/tags)
[![Stars](https://img.shields.io/github/stars/arixbit/ginblade)](https://github.com/arixbit/ginblade)
[![Forks](https://img.shields.io/github/forks/arixbit/ginblade)](https://github.com/arixbit/ginblade/fork)
[![Open Issues](https://img.shields.io/github/issues/arixbit/ginblade)](https://github.com/arixbit/ginblade/issues)
[![Last Commit](https://img.shields.io/github/last-commit/arixbit/ginblade)](https://github.com/arixbit/ginblade/commits)
[![Dependabot](https://img.shields.io/badge/Dependabot-enabled-0366d6)](https://github.com/arixbit/ginblade/security/dependabot)
[![codecov](https://codecov.io/gh/arixbit/ginblade/branch/main/graph/badge.svg)](https://codecov.io/gh/arixbit/ginblade)
[English](./README.md) · [架构文档](./ARCHITECTURE.zh-CN.md)

一个**有观点、可直接运行**的 Go 后端骨架，为需要清晰分层、多进程独立部署、且能从本地开发一路验证到 CI 的服务而设计。

> 业务模块刻意保持最小——`Example` 流程仅用于演示各层如何拼装，不代表完整产品，也不代表"唯一正确的架构"。

---

## 目录

- [特性亮点](#特性亮点)
- [技术栈](#技术栈)
- [目录结构](#目录结构)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [示例 API](#示例-api)
- [异步任务](#异步任务)
- [健康检查](#健康检查)
- [测试与 CI](#测试与-ci)
- [依赖自动更新](#依赖自动更新)
- [部署注意](#部署注意)
- [常见问题](#常见问题)

---

## 特性亮点

- **三个独立进程**：HTTP API、Asynq 异步任务 Worker、数据库迁移，可分别部署、独立扩缩容。
- **清晰分层**：`handler → service → repository` 请求流，接口在消费方（service）定义，依赖倒置。
- **手写依赖注入 + 集中生命周期管理**：`bootstrap.Registry` 统一持有 DB / Redis / JWT / 队列资源，进程退出时统一释放。
- **基础设施可选**：Redis 与 JWT 均为可选依赖，未配置时自动降级（路由不注册、健康检查如实上报），本地零依赖也能跑起来。
- **可观测性内建**：请求级 trace_id 贯穿 HTTP 与异步任务（X-Request-ID 透传/生成、任务 payload 携带、zap 结构化日志）。
- **完整交付链路**：多阶段非 root 容器镜像、本地 Docker Compose 全套、CI 覆盖单测 / race / 集成 / lint / 容器冒烟。

## 技术栈

| 领域 | 选型 |
|---|---|
| 语言 / 工具链 | Go 1.25（toolchain go1.25.5，Go ≥ 1.21 支持自动工具链切换） |
| Web 框架 | Gin v1.10 |
| ORM / 数据库 | GORM v1.30 + PostgreSQL（pgx v5） |
| 异步任务 | Asynq v0.26（基于 Redis） |
| 缓存 | go-redis v9 |
| JWT | golang-jwt/v5（HS256） |
| 日志 | zap（JSON / console，含 trace_id） |
| 参数校验 | go-playground/validator/v10 |
| 容器 | 多阶段 Dockerfile（golang:1.25.5-alpine 构建 → alpine:3.22 运行，非 root `app` 用户） |
| 编排 | Docker Compose（Postgres 17 + Redis 7 + migrate + api + worker） |

## 目录结构

```
├── cmd/                    # 三个可执行进程入口
│   ├── api/                #   HTTP API 进程
│   ├── worker/             #   Asynq 任务进程
│   └── migrate/            #   数据库迁移（GORM AutoMigrate）
├── config/                 # 环境变量加载 + 类型化配置
├── internal/
│   ├── bootstrap/          # 进程级资源初始化与生命周期（Registry）
│   ├── handler/            # HTTP 层：绑定/校验请求、调 service、统一响应
│   ├── service/            # 业务层：定义 repository/queue 接口（依赖倒置）
│   ├── repository/         # 持久化：GORM + context 级事务（InTx/WithTx）
│   ├── model/              # GORM 表模型（examples）
│   ├── router/             # 路由注册（按配置自动跳过可选模块）
│   ├── middleware/         # trace 日志、恢复、超时、CORS、JWT、IP 限流
│   ├── task/               # 异步任务定义（payload、类型、构造）
│   ├── taskqueue/          # 队列边界封装（可用性检查、enqueue）
│   ├── worker/             # Asynq 服务端 + 任务处理器 + trace 中间件
│   └── errcode/            # 业务错误码
├── pkg/                    # 可复用基础设施
│   ├── auth/               #   通用 JWT 签发/校验
│   ├── cache/              #   Redis 客户端封装
│   ├── database/           #   GORM 初始化 + 连接池调优
│   ├── log/                #   zap 初始化 + trace_id 上下文
│   ├── response/           #   统一响应信封
│   └── validator/          #   校验错误翻译
├── tests/integration/      # 集成测试（integration build tag，需真实 PG/Redis）
├── .github/
│   ├── workflows/          # CI（test / lint / container）+ Dependabot 自动合并
│   └── dependabot.yml      # 依赖自动更新配置
├── Dockerfile
├── compose.yaml
└── Makefile
```

## 快速开始

### 方式 A：Docker Compose 一键启动（推荐）

启动 Postgres、Redis、迁移、API、Worker 全套：

```sh
make compose-up
curl http://127.0.0.1:3000/health
```

默认端口被占用时覆盖（宿主机端口）：

```sh
POSTGRES_PORT=55433 REDIS_PORT=56380 API_PORT=53000 make compose-up
```

停止（保留数据卷）：

```sh
make compose-down
```

### 方式 B：本地开发

前置要求：Go ≥ 1.25、Postgres、Redis（Redis 可选，见[配置说明](#配置说明)）。

```sh
cp .env.example .env
make migrate          # 执行数据库迁移（examples 表）
go run ./cmd/api      # 启动 API，默认 :3000
```

Redis 已配置时另开终端启动 Worker：

```sh
go run ./cmd/worker
```

> 本地 Go 版本低于 go.mod 要求时，默认 `GOTOOLCHAIN=auto` 会自动下载并使用对应工具链；Docker 构建使用 `GOTOOLCHAIN=local`，因此构建镜像内的 Go 版本必须与 go.mod 匹配。

## 配置说明

全部配置通过环境变量注入，`config.Load()` 读取并带默认值。`LoadEnv` 会加载 `cmd/<进程>/.env`，未找到时回退到根目录 `.env`。

| 变量 | 必填 | 默认值 | 说明 |
|---|---|---|---|
| `SERVER_PORT` | 否 | `:3000` | HTTP 监听地址 |
| `GIN_MODE` | 否 | `release` | Gin 运行模式 |
| `REQUEST_TIMEOUT` | 否 | `30s` | 请求超时（context 截止时间） |
| `TRUSTED_PROXIES` | 否 | 空 | 可信代理列表（逗号分隔） |
| `POSTGRES` | API 必需 | 空 | PostgreSQL DSN（如 `postgres://user:pass@127.0.0.1:5432/app?sslmode=disable`） |
| `GORM_LOG_LEVEL` | 否 | `warn` | GORM 日志级别（silent/error/info/warn） |
| `DB_MAX_IDLE_CONNS` | 否 | `15` | 连接池最大空闲连接 |
| `DB_MAX_OPEN_CONNS` | 否 | `30` | 连接池最大打开连接 |
| `DB_CONN_MAX_LIFETIME` | 否 | `30m` | 连接最大存活时间 |
| `DB_CONN_MAX_IDLE_TIME` | 否 | `5m` | 连接最大空闲时间 |
| `REDIS_ADDR` | Worker 必需 | 空 | Redis 地址（API 可选） |
| `REDIS_PASSWORD` | 否 | 空 | Redis 密码 |
| `REDIS_CACHE_DB` | 否 | `0` | 缓存使用的 DB 编号 |
| `REDIS_QUEUE_DB` | 否 | `6` | Asynq 队列使用的 DB 编号 |
| `JWT_SECRET` | 否 | 空 | JWT 签名密钥；配置后启用 auth 示例路由 |
| `JWT_ISSUER` | 否 | `ginblade` | JWT 签发者 |
| `JWT_TTL` | 否 | `24h` | Token 有效期 |
| `CORS_ALLOW_ORIGINS` | 否 | 空 | 允许的浏览器来源（逗号分隔）；空 = 不发 CORS 头 |
| `LOG_LEVEL` | 否 | `info` | 日志级别 |
| `LOG_FORMAT` | 否 | `json` | `json` 或 `console` |
| `LOG_STACKTRACE_LEVEL` | 否 | `error` | 堆栈打印级别 |
| `AUDIT_LOG_ENABLED` | 否 | `true` | 是否记录请求审计日志 |
| `AUDIT_LOG_EXCLUDE_PATHS` | 否 | 空 | 审计日志排除路径（逗号分隔，默认建议 `/health`） |
| `RATE_LIMIT_PER_MINUTE` | 否 | `0` | 每 IP 每分钟请求上限；`0` = 关闭 |

**各进程的依赖要求**：

- API：必须配置 `POSTGRES`；Redis 可选（配置后启用缓存与任务发布）；JWT 可选（配置后启用 auth 路由）。
- Worker：必须配置 `REDIS_ADDR`；Postgres 可选（仅当需要访问数据库时）。
- Migrate：需要 `POSTGRES`。

## 示例 API

基础路径 `/api/v1`。

| 方法 | 路径 | 说明 | 认证 |
|---|---|---|---|
| GET | `/health` | 健康检查（真实 HTTP 状态码） | 无 |
| POST | `/api/v1/auth/token` | 签发示例 JWT | 无 |
| GET | `/api/v1/auth/me` | 返回当前 subject | Bearer（仅配置 JWT_SECRET 后注册） |
| GET | `/api/v1/examples` | 分页列表 `?limit=&offset=` | 无 |
| POST | `/api/v1/examples` | 创建示例（`{"name": "..."}`） | 无 |
| POST | `/api/v1/examples/tasks` | 发布异步任务（`{"name": "..."}`） | 无 |

签发 Token：

```sh
curl -X POST http://127.0.0.1:3000/api/v1/auth/token \
  -H 'Content-Type: application/json' \
  -d '{"subject":"demo"}'
```

调用受保护接口：

```sh
curl http://127.0.0.1:3000/api/v1/auth/me \
  -H "Authorization: Bearer <access_token>"
```

发布异步任务（需 Redis）：

```sh
curl -X POST http://127.0.0.1:3000/api/v1/examples/tasks \
  -H 'Content-Type: application/json' \
  -d '{"name":"demo"}'
```

### 响应格式

统一信封：`{ "code": 0, "msg": "success", "data": ... }`

- 成功：`code = 0`。
- 业务错误：`{ "code": <错误码>, "msg": <可读消息>, "reason": <机器可读原因>, "metadata": { "trace_id": ... } }`，**约定 HTTP 200**（`/health` 除外，使用真实状态码）。

| code | reason | 含义 |
|---|---|---|
| 1001 | `INVALID_PARAMS` | 请求参数校验失败 |
| 1002 | `UNAUTHORIZED` | 未认证 / 凭证无效 |
| 1003 | `PERMISSION_DENIED` | 已认证但无权限 |
| 1004 | `TOO_MANY_REQUESTS` | 触发 IP 限流 |
| 1005 | `REQUEST_TIMEOUT` | 请求超时 |
| 9001 | `INTERNAL_ERROR` | 未预期的服务端错误 |
| 9002 | `DATABASE_ERROR` | 数据库操作失败 |
| 9003 | `QUEUE_UNAVAILABLE` | 队列未配置 |
| 9004 | `QUEUE_ERROR` | 任务发布失败 |

## 异步任务

基于 Asynq + Redis，API 进程发布任务，Worker 进程消费：

- **队列**：三个优先级队列 `critical:6 / default:3 / low:1`，并发 10。
- **重试**：示例任务 `MaxRetry(5)`，指数退避 `5s × 2^n`（上限 1 小时），失败由 ErrorHandler 记录。
- **追踪**：发布侧把 `trace_id` 写入任务 payload，Worker 侧中间件恢复 trace 并记录任务生命周期（task_id / queue / retry_count）。

## 健康检查

`GET /health` 返回各依赖状态，任何必需依赖不可用即返回 **503**：

```json
{ "status": "ok", "checks": { "postgres": "ok", "redis": "ok" } }
```

- Postgres 未配置 → `not_configured`（不健康）；已配置但 ping 失败 → `unavailable`。
- Redis 未配置 → `not_configured`（**不算不健康**，因 Redis 可选）。

## 测试与 CI

### 常用 Make 目标

| 目标 | 说明 |
|---|---|
| `make ci` | 本地完整检查：格式、mod 一致性、vet、race 测试、构建 |
| `make test` / `make test-race` | 单元测试 / 竞态检测 |
| `make integration-up` | 启动隔离的 Postgres + Redis 测试服务（独立 Compose 项目） |
| `make test-integration` | 运行集成测试（真实 PG/Redis） |
| `make integration-down` | 移除集成测试服务 |
| `make lint` | golangci-lint |
| `make build` | 构建 api / worker / migrate 三个二进制到 `bin/` |
| `make docker-build` | 构建容器镜像 |
| `make compose-up / compose-down / compose-logs` | 本地全套编排 |
| `make clean` | 清理构建产物 |

集成测试带 `integration` build tag，需要 `TEST_POSTGRES_DSN`、`TEST_REDIS_ADDR`、`TEST_REDIS_CACHE_DB`、`TEST_REDIS_QUEUE_DB`（Makefile 提供安全本地默认值；多工作区并行时用 `INTEGRATION_PROJECT`、`INTEGRATION_POSTGRES_PORT`、`INTEGRATION_REDIS_PORT` 覆盖）。

### CI（GitHub Actions）

- **test**：单元检查 + 集成测试（内置 Postgres/Redis 服务容器）。
- **lint**：golangci-lint v2。
- **container**：校验 Compose 配置 → 构建镜像 → 校验非 root 运行 → 启动全套栈 → 冒烟测试（health、创建数据、发布任务、Worker 日志确认消费）。

## 依赖自动更新

- `.github/dependabot.yml` 已启用 **Dependabot**（每周一 08:00 Asia/Shanghai）：Go 模块按组出 PR（`golang.org/x/*` 一组、`gorm.io/*` 一组、其余 minor/patch 一组，major 单独出 PR），外加 Docker 镜像与 GitHub Actions 更新。
- `.github/workflows/dependabot-auto-merge.yml` 对 Dependabot PR 自动启用 auto-merge：**minor/patch 在 CI 全绿后自动合并；major 更新跳过自动合并留给人 review**。
- 前提：仓库需开启 "Allow auto-merge"，且 main 分支配置 ruleset 要求 `test` / `lint` / `container` 检查通过——构建失败会阻止合并。

## 部署注意

- **镜像**：多阶段构建、非 root `app` 用户运行、`EXPOSE 3000`；`/usr/local/bin/` 下包含 `api`、`worker`、`migrate` 三个二进制。
- **Swagger**：骨架未启用，部署不需要 `swag init`。
- **JWT**：生产环境务必替换 `JWT_SECRET`（本地示例值仅用于开发）。
- **CORS**：`CORS_ALLOW_ORIGINS` 为逗号分隔白名单，空值不发 CORS 头。
- **错误约定**：业务错误统一 HTTP 200 + 信封编码（便于网关统一处理）；`/health` 使用真实状态码。
- **数据卷**：`compose-down` 保留数据；跨大版本升级 Postgres/Redis 镜像需注意数据卷兼容（参考 pg_upgrade / 重建卷）。

## 常见问题

**Q：不配 Redis / JWT 能跑吗？**
能。API 进程仅强依赖 Postgres；未配置 Redis 时任务发布接口返回 `QUEUE_UNAVAILABLE`（9003），未配置 JWT 时 auth 路由不注册。

**Q：为什么业务错误返回 HTTP 200？**
骨架约定：业务错误用信封内的 `code` 表达，HTTP 状态码仅用于传输层语义（健康检查、超时等）。这是可配置的团队约定，按需调整即可。

**Q：如何新增一个业务模块？**
照抄 `Example` 五件套：`model` → `repository`（含接口）→ `service` → `handler` → `router` 注册；异步流程再加 `task` 定义与 `worker` 处理器，并在 `internal/server.go` / `internal/worker.go` 中接线。

**Q：本地 Go 版本比 go.mod 低怎么办？**
默认 `GOTOOLCHAIN=auto` 会自动下载并使用所需工具链；或执行 `go get go@<版本>` 手动升级 go.mod 的 `go`/`toolchain` 行（Docker 构建需同步更新 builder 镜像版本）。
