# 多语言（i18n）集成指南

本分支内置了 API 提示语的多语言：响应信封里的 `msg` 按请求者语言下发，接口契约的其余部分保持不变。（`main` 不内置多语言，本分支是可运行的参考实现。）本文说明它如何工作、语言文件放在哪里以及为什么，以及如何新增语言和文案。

## 核心原则：locale 只进消息层，不进 errcode

这是本指南最重要的一条。`errcode.Error` 的 `reason`（`INVALID_PARAMS` 等）是**稳定的机器可读契约**——前端/SDK 靠它做 switch 判断错误类型。一旦 reason 随语言变化，所有下游的判断逻辑都会失效。

因此：

- `reason` 保持语言无关，永远是 `INVALID_PARAMS`。
- `msg` 承载可变文案，按请求 locale 翻译。
- 响应 JSON 结构不变：`{ code, msg, reason, data, metadata }`。

把"契约"和"文案"分离，是不破坏现有接口就能加上 i18n 的关键。

## 工作方式

```
请求 ─▶ Locale 中间件 ─▶ c.Set("localizer", tr.Localizer(?lang, Accept-Language))
                                  │
handler/中间件报错 ─▶ pkg/response  ─▶ localizer.Error(reason)          ─▶ msg
参数校验失败       ─▶ pkg/validator ─▶ localizer.Validation(tag, ...) ─▶ msg
```

| 组成 | 职责 |
|---|---|
| `internal/locales/` | 语言文件（`en.json`、`zh-CN.json`），用 `//go:embed` 编译进二进制 |
| `pkg/i18n` | 从任意 `fs.FS` 加载文件、匹配语言、查找文案；不依赖 Gin |
| `internal/middleware/locale.go` | 解析请求语言，存入 `Localizer` |
| `pkg/response`、`pkg/validator` | 读取 `Localizer` 填充 `msg` |
| `internal/bootstrap` | `i18n.New(locales.FS, cfg.I18n.DefaultLocale)` → `Registry.I18n` |

`Locale` 挂在 `TraceLogger` 之后，所以 `Recovery`、`RateLimit`、`BearerAuth` 写出的错误同样会被翻译。handler 完全感知不到 locale。

## 语言文件放在哪里，为什么

JSON 文件放在 `internal/locales/`，由同目录的 `locales.go` embed。选这里是因为三个约束：

1. **`//go:embed` 不能引用上级目录**（`../locales/*.json` 是非法模式），所以负责 embed 的包必须和文件在同一目录。
2. **生产镜像里只有二进制。** `Dockerfile` 只复制 `/out/`，GoReleaser 只打包 `LICENSE` 和 `README.md`。用 embed，构建、部署、工作目录都不用改。
3. **文案的 key 是这个应用自己的契约**，所以文件属于 `internal/`（和 `internal/errcode` 放在一起），可复用的机制放在 `pkg/i18n`——和 `pkg/response` + `internal/errcode` 的分法一致。

如果放在根目录 `locales/`，要么在模块根目录加一个 Go 包（文案变成模块的公开 API），要么运行时读磁盘。`pkg/i18n` 只接收 `fs.FS`，将来如果需要不重新部署就改文案，在 bootstrap 里把 `locales.FS` 换成 `os.DirFS(dir)` 即可，其他代码不用动。

## 文件格式

每种语言一个文件，文件名用 BCP 47 标签（`en.json`、`zh-CN.json`）：

```json
{
  "errors": {
    "INVALID_PARAMS": "请求参数错误",
    "default": "操作失败"
  },
  "validation": {
    "required": "{field} 不能为空",
    "min": "{field} 至少为 {param}",
    "default": "{field} 格式不正确"
  }
}
```

- `errors` 的 key 用 errcode 的 **reason**，不用数字 code：可读，而且 reason 本来就是稳定契约。
- `validation` 的 key 用 validator 的 tag；`{field}`、`{param}` 是命名占位符，翻译时可以随意调整语序。
- 每一块里的 `default` 兜底没有单独条目的 reason 或 tag。

查找顺序：请求语言里的 key → 默认语言里的 key → 请求语言里的 `default` → 默认语言里的 `default` → key 本身。路由上没有 `Locale` 中间件时，`msg` 就是原始 reason。

## 语言解析

优先级：`?lang=` query > `Accept-Language` header > `I18N_DEFAULT_LOCALE`（默认 `en`）。匹配用 `golang.org/x/text/language`，所以会遵守 `q` 权重，`zh` 会匹配到 `zh-CN`，`en-GB` 会匹配到 `en`，排第一的语言不支持时会继续尝试下一个。

注意 `zh-TW`、`zh-HK` 也会匹配到 `zh-CN`（现有语言里最接近的）。要提供繁体中文，加一个 `zh-TW.json` 即可。

## 常见操作

- **新增语言** —— 把 `en.json` 复制成 `internal/locales/<标签>.json` 并翻译值，会被自动加载。
- **为新的 errcode 加文案** —— 在**每个**语言文件的 `errors` 里加上这个 reason。漏掉任何一个文件，CI 都会失败（见下文）。
- **修改默认语言** —— 设置 `I18N_DEFAULT_LOCALE`。如果没有对应的语言文件，启动会直接失败。

## 测试

- `pkg/i18n` —— 基于内存 `fstest.MapFS` 的表驱动测试：加载错误、语言匹配、每一级回退。
- `internal/locales` —— 加载真实语言文件，并断言**每个文件的 key 与 `en.json` 完全一致**。漏翻一条，CI 会报 `zh-CN.json is missing [errors.QUEUE_ERROR]`，而不是在线上悄悄回退。
- `internal/middleware` —— 覆盖 query、header、默认三种来源；`reason` 始终不变。
- `internal/server_test.go` —— 经过 `NewServer` 的端到端测试：`zh-CN` 请求发送空 body，得到 `msg: "name 不能为空"`。

## 已知限制

- 成功响应的 `msg` 仍是 `"success"`。
- 不属于校验错误的绑定错误（比如 JSON 格式错误）仍然返回 Go 的原始错误文本。
- 校验提示里的字段名是结构体字段名的小写形式，没有翻译。
- 修改文案需要重新构建（或按上文换成 `os.DirFS`）。
- worker 不做多语言。如果任务需要，参照 `trace_id` 的做法把 locale 写进任务载荷。

## 不建议的做法

- **把 locale 塞进 `errcode.Error`。** 让 reason 随语言变，破坏机器可读契约。
- **在 handler 内逐个翻译。** 与 `trace_id` 范式不一致，handler 不应感知 locale 解析。
- **默认从工作目录读取语言文件。** 镜像里只有二进制；先用 embed，确实需要热更新时再换成 `os.DirFS`。
