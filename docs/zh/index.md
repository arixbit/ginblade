# GinBlade

<div class="gb-hero" markdown>

<div class="gb-badges">
  <img src="https://github.com/arixbit/ginblade/actions/workflows/ci.yml/badge.svg" alt="CI">
  <img src="https://img.shields.io/github/go-mod/go-version/arixbit/ginblade?label=Go" alt="Go">
  <img src="https://img.shields.io/github/stars/arixbit/ginblade" alt="Stars">
  <img src="https://codecov.io/gh/arixbit/ginblade/branch/main/graph/badge.svg" alt="codecov">
</div>

# 用*清晰分层*构建服务 { }

一个**有观点、可直接运行**的 Go 后端骨架，为需要显式分层、多进程独立部署、
且能从本地开发一路验证到 CI 的服务而设计。

<div class="gb-btns">
  <a class="gb-btn gb-btn--primary" href="https://github.com/arixbit/ginblade">★ 在 GitHub 上 Star</a>
  <a class="gb-btn gb-btn--ghost" href="#快速上手">-> 快速上手</a>
</div>

</div>

---

<div class="gb-section-label">特性</div>

## 为什么选 GinBlade

<div class="gb-grid">

<div class="gb-card" markdown>
### <span class="gb-dot"></span> 进程分离

API、Asynq worker、数据库迁移是三个独立二进制，可独立部署、扩缩容、发版。
</div>

<div class="gb-card" markdown>
### <span class="gb-dot"></span> 显式分层

应用流向为 `handler -> service -> repository`。依赖指向内层，外层依赖被调用层定义的接口。
</div>

<div class="gb-card" markdown>
### <span class="gb-dot"></span> 手写依赖注入

不使用 DI 框架。资源在 `bootstrap` 中组装，以结构体显式向下传递，依赖图在一处可见。
</div>

<div class="gb-card" markdown>
### <span class="gb-dot"></span> 基础设施可选

Redis 与 JWT 均为可选。未配置时对应路由不注册、健康检查如实上报 `not_configured`，其余功能正常。
</div>

<div class="gb-card" markdown>
### <span class="gb-dot"></span> 可验证的交付路径

单元、竞态、集成、lint、容器冒烟测试在 CI 中运行。多阶段非 root 镜像 + 完整 Compose 栈。
</div>

<div class="gb-card" markdown>
### <span class="gb-dot"></span> 框架可替换

只有 HTTP 壳层依赖 Gin，`handler` 以下全部是纯 Go——换路由框架不动业务逻辑。
</div>

</div>

---

<div class="gb-section-label">快速上手</div>

## 30 秒跑起来

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

<div class="gb-section-label">结构</div>

## 项目布局

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

<div class="gb-section-label">下一步</div>

## 深入了解

<div class="gb-next">
  <a href="architecture.md"><div class="gb-next-label">📖 阅读</div><div class="gb-next-title">架构文档 - 进程模型、分层、生命周期</div></a>
  <a href="i18n-guide.md"><div class="gb-next-label">🌐 指南</div><div class="gb-next-title">多语言 - 按请求者语言下发提示语</div></a>
  <a href="https://github.com/arixbit/ginblade"><div class="gb-next-label">⚡ 源码</div><div class="gb-next-title">GitHub - 克隆、Star、贡献</div></a>
</div>
