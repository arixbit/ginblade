# GinBlade

[![CI](https://github.com/arixbit/ginblade/actions/workflows/ci.yml/badge.svg)](https://github.com/arixbit/ginblade/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/arixbit/ginblade?label=Go)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/arixbit/ginblade)](https://goreportcard.com/report/github.com/arixbit/ginblade)
[![Version](https://img.shields.io/github/v/tag/arixbit/ginblade?label=version)](https://github.com/arixbit/ginblade/tags)
[![Stars](https://img.shields.io/github/stars/arixbit/ginblade)](https://github.com/arixbit/ginblade)
[![codecov](https://codecov.io/gh/arixbit/ginblade/branch/main/graph/badge.svg)](https://codecov.io/gh/arixbit/ginblade)

---

一个**有观点、可直接运行**的 Go 后端骨架，为需要清晰分层、多进程独立部署、且能从本地开发一路验证到 CI 的服务而设计。

:octicons-star-24: [在 GitHub 上 Star](https://github.com/arixbit/ginblade){ .md-button .md-button--primary }
:octicons-rocket-24: [快速上手](#_2){ .md-button }

---

## 特性

:material-package-variant-closed: **进程分离，而非单体**

:   API、Asynq worker、数据库迁移是三个独立二进制，可独立部署、扩缩容、发版。

:material-layers-triple: **显式分层**

:   应用流向为 `handler -> service -> repository`。依赖指向内层，外层依赖被调用层定义的接口。

:material-hand-coin: **手写依赖注入**

:   不使用 DI 框架。资源在 `bootstrap` 中组装，以结构体显式向下传递，依赖图在一处可见。

:material-database-cog: **基础设施可选**

:   Redis 与 JWT 均为可选。未配置时对应路由不注册、健康检查如实上报 `not_configured`，其余功能正常。

:material-test-tube: **可验证的交付路径**

:   单元、竞态、集成、lint、容器冒烟测试在 CI 中运行。多阶段非 root 容器镜像 + 完整的本地 Docker Compose 栈。

---

## 快速上手

启动 Postgres、Redis、迁移、API 与 worker：

```sh
make compose-up
curl http://127.0.0.1:3000/health
```

停止并保留数据卷：

```sh
make compose-down
```

不用 Docker，本地运行：

```sh
cp .env.example .env
make migrate
go run ./cmd/api
```

---

## 结构

| 路径 | 用途 |
|------|------|
| `cmd/api` | HTTP API 进程 |
| `cmd/worker` | Asynq worker 进程 |
| `cmd/migrate` | GORM 迁移入口 |
| `config` | 环境变量加载与类型化配置 |
| `internal/bootstrap` | 进程级资源初始化 |
| `internal` | 应用组装、路由、中间件、分层 |
| `pkg` | 可复用基础设施助手（JWT、缓存、日志等） |

---

## 下一步

- :material-book-open-page-variant: [架构文档](architecture.md) - 进程模型、分层、生命周期
- :material-translate: [多语言集成指南](i18n-guide.md) - 如何按请求者语言下发提示语
- :material-github: [GitHub 源码](https://github.com/arixbit/ginblade)
