# 事务

怎么让多行数据库写入**要么全成功、要么全失败**。

骨架只提供一个事务原语 `repository.TxRunner`，`Wallet` 模块是它的完整范例。本文讲清三件事：什么时候需要事务、代码到底怎么写、以及为什么 repository 层一行都不用改。

## 什么时候需要事务

当一次请求要写**多于一行或一张表**，并且"只写进去一半"是 **bug 而不是小事**时，就需要事务。

转账是最典型的例子：钱从一个账户扣出、进到另一个账户。如果扣款提交了、入账没提交，钱就凭空消失了；中途被打断，账就再也对不平。

如果一次只写一行，那这里你什么都不用做——GORM 已经给单个 `Create`/`Update` 包了事务。

## 原语

`internal/service/tx.go` 定义了 service 所依赖的接口：

```go
type TransactionRunner interface {
	InTx(ctx context.Context, fn func(context.Context) error) error
}
```

`internal/repository/tx.go` 提供实现 `TxRunner`，它本质就是绑定了一个 `*gorm.DB` 的 `InTx`：

```go
func NewTxRunner(db *gorm.DB) *TxRunner
```

`internal/server.go` 用共享的数据库句柄构建它并注入：

```go
walletService := service.NewWalletService(
	repository.NewWalletRepository(db),
	walletCache(reg.Cache),
	repository.NewTxRunner(db),
)
```

你的 service 持有接口，永远不碰 GORM：

```go
type WalletService struct {
	repo WalletRepository
	tx   TransactionRunner
}
```

## 回调怎么写

把多行操作包进 `InTx`。回调里拿到的是 `txCtx`，事务就挂在它身上：

```go
func (s *WalletService) Transfer(ctx context.Context, req *TransferReq) (*TransferRes, error) {
	if req.FromID == req.ToID {
		return nil, errcode.InvalidParams
	}

	err := s.tx.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Debit(txCtx, req.FromID, req.Amount); err != nil {
			return err
		}
		if err := s.repo.Credit(txCtx, req.ToID, req.Amount); err != nil {
			return err
		}
		return s.repo.CreateTransferRecord(txCtx, req.FromID, req.ToID, req.Amount)
	})
	if err != nil {
		return nil, errcode.DatabaseError
	}
	return &TransferRes{Success: true}, nil
}
```

三条规则决定了它是否正确：

1. **往下传 `txCtx`，绝不传外层 `ctx`。** 把外层 context 交给 repository，那条语句就会悄悄跑到事务外面执行，回滚时它不会跟着回滚。
2. **返回 `nil` 即提交，返回任何 error 即回滚。** 你不需要自己调 `Commit` 或 `Rollback`。
3. **遇到第一个失败就返回。** 一步失败后应立刻 return，而不是继续写下去、事后再想办法收拾。

## 为什么 repository 不用改

repository 从头到尾都不知道事务的存在。每个方法都通过 `dbFromContext` 取句柄：context 里有事务就返回事务，没有就返回普通连接：

```go
// internal/repository/tx.go
func dbFromContext(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx := txFromContext(ctx); tx != nil {
		return tx
	}
	return db
}
```

所以 `Debit` 无论是在转账中被调用，还是被单独调用，写法完全一样：

```go
func (r *WalletRepository) Debit(ctx context.Context, id uint64, amount int64) error {
	res := dbFromContext(ctx, r.db).WithContext(ctx).
		Model(&model.Wallet{}).
		Where("id = ? AND balance >= ?", id, amount).
		UpdateColumn("balance", gorm.Expr("balance - ?", amount))
	// ...
}
```

有两点值得记住：

- **repository 方法保持单一职责。** 它们是原子操作，哪些操作要一起走由 service 决定。
- **service 里绝不要去找 `dbFromContext`。** 它之所以不导出，正是因为 service 只应该用 `TransactionRunner` 说话。

## 嵌套事务会加入外层

在 context 里已经有事务时再调 `InTx`，不会另开一个事务，而是直接在已有事务里执行回调。因此一个 service 方法可以安全地调用另一个内部也用了 `InTx` 的 service 方法：

```go
s.tx.InTx(ctx, func(txCtx context.Context) error {
	return otherService.DoSomething(txCtx) // 内部同样使用 InTx
})
```

内层调用会加入外层事务；任意一层失败，整体一起回滚。

## 错误映射

`InTx` 会把回调返回的 error 原样交还给你。仓储层的失败是哨兵错误，因此 service 可以据此挑选业务错误码：

```go
if err != nil {
	switch {
	case errors.Is(err, repository.ErrInsufficientBalance):
		return nil, errcode.InsufficientBalance
	case errors.Is(err, repository.ErrWalletNotFound):
		return nil, errcode.NotFound
	default:
		return nil, errcode.DatabaseError
	}
}
```

请用 `errors.Is` 而不是 `==`，因为错误在向上传递的过程中可能被包装过。

## 把判断放进 SQL

事务能让一组写入变原子，但它**本身不能**阻止两个并发事务各自基于同一个过期余额操作。`Debit` 把判断挪进 `WHERE` 条件来堵住这个口子：

```sql
UPDATE wallets SET balance = balance - $1 WHERE id = $2 AND balance >= $1
```

余额不足时这条更新匹配不到任何行，`RowsAffected` 为 0，仓储层于是返回 `ErrInsufficientBalance`。"检查 + 更新"因此是一条原子语句。换成"先读余额、再比较、然后写"就会产生竞态。

## 去哪儿看它跑起来

| 内容 | 位置 |
|---|---|
| service 层编排 | `internal/service/wallet.go` |
| service 依赖的接口 | `internal/service/tx.go` |
| 事务原语 | `internal/repository/tx.go` |
| 装配 | `internal/server.go` |
| mock 编排测试 | `internal/service/wallet_test.go` |
| 生成的 SQL 测试（GORM dry run） | `internal/repository/wallet_test.go` |
| 针对真实 Postgres 的提交/回滚测试 | `tests/integration/wallet_test.go` |
