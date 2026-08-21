# 多语言（i18n）集成指南

GinBlade 不内置多语言。所有面向用户的文案都是英文硬编码，且对每个请求一视同仁。本文面向希望让 API 按请求者语言下发提示语的开发者，说明如何在不破坏现有分层与接口契约的前提下把它加进来。

## 现状

- `pkg/response/response.go` 的 `messageFor(reason)` 是一个纯英文 `switch`，按 errcode reason 返回文案。
- `pkg/validator/validator.go` 的 `HandleValidatorError` 同样硬编码（`%s is required` 等），是字段校验消息的出口。
- `internal/errcode` 的 `Error` 只有 `code` + `reason`，没有 locale 维度。
- 没有任何中间件读取请求语言。

## 核心原则：locale 只进消息层，不进 errcode

这是本指南最重要的一条。`errcode.Error` 的 `reason`（`INVALID_PARAMS` 等）是**稳定的机器可读契约**——前端/SDK 靠它做 switch 判断错误类型。一旦 reason 随语言变化，所有下游的判断逻辑都会失效。

因此：

- `reason` 保持语言无关，永远是 `INVALID_PARAMS`。
- `msg` 承载可变文案，按请求 locale 翻译。
- 响应 JSON 结构不变：`{ code, msg, reason, data, metadata }`。

把"契约"和"文案"分离，是不破坏现有接口就能加上 i18n 的关键。

## 集成步骤

下列代码片段展示推荐形态，并非可直接落地的完整实现。

### 1. 新增 `pkg/i18n` 包

放翻译表与查找逻辑，纯函数、不依赖 gin，便于单测：

```go
package i18n

type Translator struct {
    defaultLocale string
    messages    map[string]map[string]string // locale -> reason -> 文案
    fieldMsgs   map[string]map[string]string // locale -> validator tag -> 模板
}

func New(defaultLocale string, messages, fieldMsgs map[string]map[string]string) *Translator { /* ... */ }

// Translate 按 locale 查找 errcode reason 的文案；locale 缺失回退 default，再缺失回退 reason 本身。
func (t *Translator) Translate(locale, reason string) string { /* ... */ }

// TranslateField 按 locale + validator tag 查模板并填入 field/param。
func (t *Translator) TranslateField(locale, field, tag, param string) string { /* ... */ }
```

文案存代码内 `map`，不引外部 JSON 文件、不引 `go-i18n` 库。骨架规模下 map 最直接，新增语言就是加一个 map；等文案多到需要交给非工程同事维护，再升级为外部文件，`Translator` 接口不变。

### 2. 新增 `internal/middleware/locale.go`

解析请求语言写入 `gin.Context`，与 `TraceLogger` 同范式：

```go
func Locale(defaultLocale string) gin.HandlerFunc {
    return func(c *gin.Context) {
        loc := c.Query("lang") // 1. query（便于联调时手动切换）
        if loc == "" {
            loc = parseAcceptLanguage(c.GetHeader("Accept-Language"), defaultLocale) // 2. header
        }
        if loc == "" {
            loc = defaultLocale // 3. 兜底
        }
        c.Set("locale", loc)
        c.Next()
    }
}
```

解析优先级：`?lang=` query > `Accept-Language` header > 配置默认值。query 便于联调时手动切语言，`Accept-Language` 是生产里浏览器/客户端的自然来源。`parseAcceptLanguage` 可用 `golang.org/x/text/language` 做带回退的标签匹配，或为固定的少量 locale 手写一个简易匹配器。

挂载位置：在 `TraceLogger` 之后、业务 handler 之前——与 `trace_id` 一样是每请求上下文。

### 3. 改 `pkg/response/response.go`

让 `messageFor` 从 context 取 locale，经 `Translator` 查文案：

```go
func ErrorResponse(c *gin.Context, errorCode errcode.Error, tr *i18n.Translator) Response {
    v, _ := c.Get("locale")
    loc, _ := v.(string)
    return Response{
        Code:     errorCode.Code(),
        Reason:   errorCode.Reason(), // 不变：语言无关契约
        Message:  tr.Translate(loc, errorCode.Reason()),
        Metadata: buildMetadata(c),
    }
}
```

`errcode` 包、`Error` 结构、`Reason()` 均不动。

### 4. 改 `pkg/validator/validator.go`

给 `HandleValidatorError` 加 locale 与 `Translator` 参数，走 `TranslateField`；`response.BuildValidationErrorResponse` 从 `c` 取 locale 传入。

### 5. 在 bootstrap 装配

- `internal/bootstrap/registry.go` ——给 `Registry` 加 `I18N *i18n.Translator` 字段。
- `internal/bootstrap/api.go` ——在 `InitAPI` 构造 `Translator` 并放入。
- `internal/server.go` ——把 translator 注入 response/validator 调用路径，并把 `Locale` 中间件挂到全局中间件链。

### 6. 配置

在 `config/types.go` 增加：

```go
type I18nConfig struct {
    DefaultLocale string   // 如 "en"
    Supported     []string // 如 ["en","zh"]
}
```

`Config` 结构体加 `I18n I18nConfig`。默认 locale 走配置，不写死。

## 测试

- `pkg/i18n` ——表驱动测试覆盖：命中、locale 缺失回退 default、key 缺失回退 reason 本身。
- locale 中间件 ——覆盖三级来源：query、header、默认。
- `ErrorResponse` ——给定 locale，断言 `msg` 为对应语言、`reason` 不变。

## 不建议的做法

- **把 locale 塞进 `errcode.Error`。** 让 reason 随语言变，破坏机器可读契约。
- **在 handler 内逐个翻译。** 与 `trace_id` 范式不一致，handler 不应感知 locale 解析。
- **第一天就上 `go-i18n` + JSON 文件。** 骨架阶段文案量小，过早引入文件 IO 与额外依赖只有负担。
