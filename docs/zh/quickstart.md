# 快速上手

本文回答你克隆仓库后立刻会问的四个问题：路由在哪儿写、表结构在哪儿定义、业务逻辑在哪儿写、事务怎么用。每个答案都指向真实文件，附可直接抄的代码。

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

表由 `cmd/migrate` 通过 GORM `AutoMigrate` 创建。

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

什么时候需要事务：一次业务操作要改多行数据，而且必须**全部成功或全部失败**——转账就是最典型的例子。钱从 A 账户扣出、进到 B 账户，绝不能只发生一半。

骨架提供了 `repository.TxRunner`，通过 `service.TransactionRunner` 接口注入 service。骨架里的 `WalletService` 就是一个完整的例子。

Service 持有 `TransactionRunner`，把多行操作包进它的 `InTx` 回调：

```go
// internal/service/wallet.go
type WalletService struct {
	repo WalletRepository
	tx   TransactionRunner
}

func (s *WalletService) Transfer(ctx context.Context, req *TransferReq) (*TransferRes, error) {
	if req.FromUserID == req.ToUserID {
		return nil, errcode.InvalidParams
	}

	err := s.tx.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.AddBalance(txCtx, req.FromUserID, -req.Amount); err != nil {
			return err
		}
		return s.repo.AddBalance(txCtx, req.ToUserID, req.Amount)
	})
	if err != nil {
		return nil, errcode.DatabaseError
	}
	return &TransferRes{Success: true}, nil
}
```

两次 `AddBalance` 分别是扣款和入账，都在同一个事务里：

- 扣款成功、入账失败 → 扣款回滚，转账返回错误，钱一分没动；
- 两步都成功 → 事务提交。

Repository 方法本身不用改——每个方法都走 `dbFromContext`，它会在 context 里有事务时自动用事务，没有时用普通连接：

```go
// internal/repository/wallet.go
func (r *WalletRepository) AddBalance(ctx context.Context, userID uint64, delta int64) error {
	return dbFromContext(ctx, r.db).WithContext(ctx).
		Model(&model.Wallet{}).
		Where("user_id = ?", userID).
		UpdateColumn("balance", gorm.Expr("balance + ?", delta)).Error
}
```

嵌套的 `InTx` 会加入外层事务而不是另开一个，所以事务里再套事务是安全的。

装配在 `internal/server.go`：repository 和 `TxRunner` 都从数据库连接构建，一起传给 service。

```go
walletRepository := repository.NewWalletRepository(db)
walletService := service.NewWalletService(walletRepository, repository.NewTxRunner(db))
```

## 组装

`internal/server.go` 组装 repository → service → handler。照着现有 `Example` 的接线方式把你的新组件接进去，第 1 步的路由组就生效了。
