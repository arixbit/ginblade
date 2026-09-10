# 快速上手

本文回答你克隆仓库后立刻会问的五个问题：路由在哪儿写、表结构在哪儿定义、业务逻辑在哪儿写、事务怎么用、缓存怎么用。每个答案都指向真实文件，附可直接抄的代码。

## 东西都放在哪

| 路径 | 放什么 |
|------|--------|
| `cmd/api/main.go` | HTTP 服务入口 |
| `internal/router/` | 路由定义 |
| `internal/handler/` | HTTP 请求处理：绑定参数、调用 service、写响应 |
| `internal/service/` | 业务逻辑 |
| `internal/repository/` | 数据库操作（GORM） |
| `internal/model/` | 表结构（GORM 模型） |
| `internal/errcode/` | 业务错误码 |
| `internal/task/`、`internal/worker/` | 异步任务类型与处理函数 |
| `pkg/response/` | 统一响应封装 |

一次请求的流向：`router → handler → service → repository → Postgres`。

骨架自带两条参考链路。`Example` 展示基础的 `handler → service → repository` 路径以及异步任务投递；`Wallet` 在它之上补了 `Example` 没覆盖的两件事：多行事务与 cache-aside 读。

## 1. 路由在哪儿写

文件：`internal/router/router.go`

```go
func registerExampleRoutes(r *gin.RouterGroup, deps Dependencies) {
	if deps.Example == nil {
		return
	}

	examples := r.Group("/examples")
	examples.GET("", deps.Example.List)
	examples.POST("", deps.Example.Create)
	examples.POST("/tasks", deps.Example.EnqueueTask)
}
```

在现有路由组里加一行，或新建路由组后在 `RegisterRoutes` 里调用。

## 2. 表结构在哪儿定义

文件：`internal/model/example.go`

```go
type Example struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"column:name;type:varchar(255);not null" json:"name"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;default:CURRENT_TIMESTAMP;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamp;default:CURRENT_TIMESTAMP;not null" json:"updated_at"`
}

func (Example) TableName() string {
	return "examples"
}
```

表由 `cmd/migrate` 通过 GORM `AutoMigrate` 创建。新模型要注册到那次调用里——wallet 示例就把 `model.Wallet` 和 `model.TransferRecord` 都加进去了。

## 3. 业务逻辑在哪儿写

两个地方，按顺序：

**Service（业务规则）** — `internal/service/example.go`：

```go
type ExampleService struct {
	repo ExampleRepository
}

func NewExampleService(repo ExampleRepository) *ExampleService {
	return &ExampleService{repo: repo}
}

func (s *ExampleService) Create(ctx context.Context, req *CreateExampleReq) (*model.Example, error) {
	example := model.Example{Name: req.Name}
	if err := s.repo.Create(ctx, &example); err != nil {
		return nil, errcode.DatabaseError
	}
	return &example, nil
}
```

**Handler（绑定请求、调用 service、写响应）** — `internal/handler/example.go`：

```go
func (h *ExampleHandler) Create(c *gin.Context) {
	var req service.CreateExampleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WriteValidationError(c, err)
		return
	}

	example, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		response.WriteError(c, err)
		return
	}

	response.WriteSuccess(c, example)
}
```

Service 里返回 `errcode.Xxx`，handler 负责把它变成响应信封。带 `binding:` 标签的请求/响应结构体放在使用它的 service 方法旁边。

## 4. 事务怎么用

什么时候需要事务：一次业务操作要改多行数据，而且必须**全部成功或全部失败**——转账就是最典型的例子。

简短版：把 `repository.TxRunner` 以 `service.TransactionRunner` 注入，用它的 `InTx` 回调包住多行操作，并把回调里的 `txCtx` 传给每一次仓储调用。

```go
err := s.tx.InTx(ctx, func(txCtx context.Context) error {
	if err := s.repo.Debit(txCtx, req.FromID, req.Amount); err != nil {
		return err
	}
	return s.repo.Credit(txCtx, req.ToID, req.Amount)
})
```

返回 `nil` 即提交，返回 error 即回滚。Repository 方法一行都不用改，因为每个方法都通过 `dbFromContext` 取句柄，它会自动使用 context 中携带的事务。

→ **[事务](transactions.md)** 有完整写法：回调的三条规则、为什么仓储层不用动、嵌套事务、错误映射，以及阻止并发透支的 SQL 条件。

## 5. 缓存怎么用（Cache-Aside）

Redis 是可选依赖：设了 `REDIS_ADDR` 时 `bootstrap` 会构建 `*cache.Client`，没设时同一段代码必须照常工作。钱包列表读就是示范——命中就直接返回缓存值，未命中则查库并回填缓存。

```go
// internal/service/wallet.go
func (s *WalletService) List(ctx context.Context, req *ListWalletsReq) (*ListWalletsRes, error) {
	key := s.listKey(ctx, req.Limit, req.Offset)
	if s.cache != nil {
		if cached, err := s.cache.Get(ctx, key); err == nil && cached != "" {
			var wallets []model.Wallet
			if err := json.Unmarshal([]byte(cached), &wallets); err == nil {
				return &ListWalletsRes{Wallets: wallets}, nil
			}
		}
	}

	wallets, err := s.repo.List(ctx, req.Limit, req.Offset)
	if err != nil {
		return nil, errcode.DatabaseError
	}

	if s.cache != nil {
		if raw, err := json.Marshal(wallets); err == nil {
			_ = s.cache.Set(ctx, key, string(raw), walletCacheTTL)
		}
	}
	return &ListWalletsRes{Wallets: wallets}, nil
}
```

有两点让这段代码可以直接抄：

- **缓存是一个可空的接口。** service 自己声明 `WalletCache`，`internal/server.go` 把 `*cache.Client` 适配成它，Redis 未配置时返回 `nil`。每个使用点都做了判空，所以 service 不会对 Redis 产生硬依赖，单测里可以用一个 map 顶替 Redis。
- **失效用版本号。** 写操作会 bump `wallet:list:version`，而每个列表 key 都嵌了这个版本号。因此一次 `Set` 就能让所有已缓存的列表同时失效，不需要扫描或删除 key。

## 组装

`internal/server.go` 组装 repository → service → handler。照着现有 `Example` 和 `Wallet` 的接线方式把你的新组件接进去，第 1 步的路由组就生效了。
