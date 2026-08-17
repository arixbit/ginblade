# 架构文档

本文档描述 GinBlade 的设计：进程模型、分层规则、依赖管理、请求/任务生命周期，以及如何基于该骨架扩展新的业务模块。适合任何把本仓库作为真实服务起点的开发者阅读。

## 设计原则

1. **进程分离，而非单体。** API、异步 Worker、迁移工具是三个独立二进制，可独立部署、扩缩容、发版。
2. **显式分层。** 应用流向为 `handler -> service -> repository`。依赖指向内层：外层依赖被调用层定义的接口。
3. **手写依赖注入。** 不使用 DI 框架。资源在 `bootstrap` 中组装，以结构体显式向下传递。依赖图在一处可见，无魔法。
4. **集中式生命周期。** 所有共享资源（DB 连接池、Redis 客户端、队列客户端）由 `bootstrap.Registry` 持有，关闭时统一释放。
5. **基础设施可选。** Redis 与 JWT 均为可选。未配置时，对应路由不注册、健康检查如实上报 `not_configured`，其余功能正常。
6. **框架是实现细节。** 只有 HTTP 壳层依赖 Gin，`handler` 以下全部是纯 Go。若标准库（或其他路由框架）未来更合适，HTTP 层可整体替换。

## 进程模型

```
                ┌──────────────────────────────────────────────┐
                │                    Registry                  │
                │  Cfg · DBManager · Cache · JWT · Queue       │
                └───────┬──────────────┬──────────┬────────────┘
                        │              │          │
        ┌───────────────▼───┐   ┌──────▼───────┐  │
        │  cmd/api          │   │ cmd/worker   │  │
        │  HTTP server      │   │ Asynq worker │  │
        │  (Gin)            │   │              │  │
        └───────────────┬───┘   └──────┬───────┘  │
                        │              │          │
                        ▼              ▼          ▼
                   ┌──────────┐  ┌──────────┐  ┌──────────────┐
                   │ Postgres │  │  Redis   │  │  migrations  │
                   │          │  │ (queue + │  │  (cmd/migrate)│
                   └──────────┘  │  cache)  │  └──────────────┘
                                 └──────────┘
```

- **`cmd/api`** — 提供 HTTP 服务。必需 Postgres；Redis 与 JWT 可选。
- **`cmd/worker`** — 消费 Asynq 任务。必需 Redis；Postgres 可选（仅当处理器需要访问数据库时）。
- **`cmd/migrate`** — 对示例表执行 GORM `AutoMigrate`。在 Compose 栈中作为一次性任务，先于 `api`/`worker` 启动。

每个进程遵循相同的启动序列：

```
config.LoadEnv → config.Load → bootstrap.InitRuntime → bootstrap.Init<进程>
→ app.New<Server|Worker> → 基于信号的优雅停机运行
```

## 分层结构

```
HTTP 壳层（耦合 Gin）
  handler      解析/校验请求、调用 service、写响应
  middleware   链路追踪、恢复、超时、CORS、认证、限流
  router       路由注册（未配置的可选模块自动跳过）
───────────────────────────────────────────────────────────────
应用核心（框架无关）
  service      业务规则；声明它需要的 repository/queue 接口
  repository   持久化（GORM）；context 级事务（InTx/WithTx）
  model        GORM 表模型
  task         异步任务类型与载荷
  worker       Asynq 服务端、处理器、trace 中间件
  taskqueue    队列边界封装
  errcode      业务错误码
───────────────────────────────────────────────────────────────
基础设施（pkg/，可复用）
  auth · cache · database · log · response · validator
```

规则：

- **`handler` 不直接接触数据库或队列**，只调用 service 并将错误映射为响应信封。
- **`service` 定义它消费的接口**（`ExampleRepository`、`ExampleQueue`），具体实现在 `bootstrap` 注入。这是"消费方定义接口"模式，也是 service 层无需数据库即可单元测试的原因。
- **`repository` 只认识 GORM**。事务能力通过 context 传递：`InTx` 开启事务（context 中已有事务则复用），`dbFromContext` 在 repository 内部透明使用当前事务。
- **`worker` 处理器接收 `Deps` 结构体**（DB、缓存、Redis 客户端、队列），与 handler 层的注入风格一致。

## 依赖注入与生命周期

`internal/bootstrap` 是组合根：

- `InitAPI` / `InitWorker` 构建共享资源并返回 `*Registry`。
- 可选资源在缺少对应配置时返回 `nil`（无 Redis 地址时 `initCache` 返回 nil，无 JWT 密钥时 `initAuth` 返回 nil）。
- `Registry.Close()` 释放所有资源，用 `errors.Join` 聚合错误——部分失败也会关闭其余资源。

HTTP 处理器与 worker 依赖随后分别在 `internal/server.go` 与 `internal/worker.go` 中组装。

## HTTP 请求生命周期

```
client
  │  X-Request-ID（可选）
  ▼
TraceLogger  生成/透传 trace id；完成后记录审计日志
Recovery      panic → errcode.InternalError 响应
Timeout      附加 context 截止时间（REQUEST_TIMEOUT）
CORS         白名单校验，OPTIONS 短路
RateLimit    内存版按 IP 令牌桶（启用时）
  ▼
router → handler → service → repository → Postgres
  ▼
响应信封 { code, msg, reason, data, metadata{trace_id} }
```

约定：

- 成功：`code = 0`，HTTP 200。
- 业务错误：`{ code, msg, reason, metadata }`，**按约定 HTTP 200**（信封内的 code 是语义来源）。
- `/health` 例外：使用真实 HTTP 状态码，必需依赖不可用时返回 503。

## 异步任务生命周期

1. **发布** — `service` 用 `task.NewExampleTask(name, traceID)` 构建任务，调用 `queue.Enqueue(ctx, task)`。请求 context 中的 `trace_id` 写入载荷。
2. **消费** — worker 的 `ServeMux` 按任务类型路由到处理器。`TraceMiddleware` 从载荷恢复 `trace_id`（没有则生成任务域 trace），并记录开始/结束/失败事件及任务 id、队列名、重试次数。
3. **重试** — `MaxRetry(5)`，指数退避 `5s × 2^n`，上限 1 小时；`ErrorHandler` 记录失败。

三个优先级队列：`critical:6 / default:3 / low:1`，并发 10。

## 配置

所有配置来自环境变量，`config.Load()` 一次性读取并带默认值。`config.LoadEnv` 加载 `cmd/<进程>/.env`，回退到仓库根目录 `.env`。完整变量表见 `README.zh-CN.md`。

## 扩展：新增业务模块

参照 `Example` 流程。同步功能：

1. `internal/model/` — GORM 模型。
2. `internal/repository/` — 提供 service 所需方法的 repository；需要事务时使用 `InTx`/`dbFromContext`。
3. `internal/service/` — 业务逻辑；在此声明 repository 接口；暴露带校验标签的请求/响应结构体。
4. `internal/handler/` — 绑定/校验、调用 service、写响应。
5. `internal/router/` — 注册路由。
6. `internal/server.go` — 组装 repository → service → handler。

异步功能，额外：

7. `internal/task/` — 任务类型常量、载荷结构体、构造函数。
8. `internal/worker/handler.go` — 处理器；在 `RegisterHandlers` 中注册。
9. `internal/service/` — 发布任务（尊重 `queue.Available()`）。

## 框架耦合面（为什么换掉 Gin 成本很低）

框架边界是刻意设计的。以下包引用 `gin`：

```
internal/handler   internal/middleware   internal/router
internal/server.go pkg/response
```

以下包**不**引用：

```
internal/service  internal/repository  internal/model
internal/task     internal/taskqueue   internal/worker
internal/bootstrap
pkg/auth  pkg/cache  pkg/database  pkg/log  pkg/validator
```

业务规则、持久化、任务处理与基础设施助手均与框架无关。若标准库路由（Go 1.22+ 的 `http.ServeMux` 已支持方法匹配与路径通配符）或其他框架成为更优选择，迁移只需重写 HTTP 壳层——handler、middleware、router 与 `pkg/response`——应用核心保持不变。
