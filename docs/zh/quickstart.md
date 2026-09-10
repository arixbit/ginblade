# 快速上手

本文回答你克隆仓库后立刻会问的三个问题：路由在哪儿写、表结构在哪儿定义、业务逻辑在哪儿写。每个答案都指向真实文件，附可直接抄的代码。

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

## 组装

`internal/server.go` 组装 repository → service → handler。照着现有 `Example` 的接线方式把你的新组件接进去，第 1 步的路由组就生效了。
