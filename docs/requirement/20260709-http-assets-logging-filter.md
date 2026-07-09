# HTTP 静态资源访问日志过滤
最后修改时间: 2026-07-09 16:20:00

Flow mode: light / 轻量模式
Stage: Requirement / 需求
Review status: Accepted

## Background

当前后端 access log 会记录前端静态资源请求。浏览器访问页面时，`/assets/` 下的 JavaScript、CSS、图片、字体等构建产物会产生大量成功请求日志，这类 `200` 日志诊断价值低，容易淹没 API、错误和异常请求日志。

PomeloOrbit-go 的 `4cc32671` 可作为需求边界参考：只借鉴“成功静态资源 access log 降噪”的意图，不照搬其配置命名、实现结构或其它无关改动。

## Goal

- 默认过滤 `/assets/` 开头、常见静态资源扩展名结尾、最终 HTTP status 为 `200` 的 access log。
- 增加配置开关，默认启用；关闭后保留现有 access log 行为。
- 过滤目标仅是日志输出降噪，不改变请求处理、响应内容、静态文件服务、SPA fallback 或 API 行为。
- 过滤规则应集中在日志 middleware 或其配置层，不把路径、扩展名判断散落到业务处理逻辑。

## Non-goal

- 不过滤 API 请求日志。
- 不过滤 `/assets/` 下非 `200` 请求日志，例如 `404`、`500`、`304`。
- 不改变前端请求发起方式、前端 runtime config、Docker 配置或 Cloud/local API origin 选择。
- 不引入复杂的前端请求分类、dev token proxy 或代理路由配置。
- 不照搬 PomeloOrbit-go 的配置前缀、settings 服务或其它项目特定实现。

## User scenarios

- 作为开发者，我访问前端页面时，不希望大量 `/assets/*.js`、`/assets/*.css`、`/assets/*.jpg` 这类成功静态资源请求刷屏。
- 作为维护者，我仍然希望 API 请求和失败的静态资源请求保留日志，便于排查问题。
- 作为部署者，如果需要临时排查完整静态资源访问链路，我可以关闭过滤开关恢复完整 access log。

## Acceptance

- `/assets/` 开头且扩展名属于常见静态资源类型的请求，在最终响应为 `200` 时不输出 access log。
- 静态资源扩展名集合至少包含 `.js`、`.mjs`、`.css`、`.map`、`.jpg`、`.jpeg`、`.png`、`.gif`、`.svg`、`.ico`、`.webp`、`.avif`、`.woff`、`.woff2`、`.ttf`、`.otf`、`.eot`、`.wasm`。
- 增加配置开关，默认值为启用。
- 关闭配置开关后，`/assets/` 下成功静态资源请求仍按现有规则输出 access log。
- `/assets/` 下静态资源返回非 `200` 时仍输出 access log。
- 非 `/assets/` 路径即使扩展名相同，也不被该规则过滤。
- API 路径不受影响，成功和失败请求都按现有规则记录。
- 过滤逻辑不改变 handler 执行、响应状态码、响应体或静态文件服务行为。
- 测试断言应表达业务语义：成功静态资源日志被降噪、关闭开关后日志恢复、失败和 API 日志被保留。

## Open questions

暂无需要用户确认的未决事项。

## Decisions

- 范围收敛为 `/assets/` 静态资源 `200` access log 过滤。
- 静态资源扩展名集合包含常见前端构建资源：脚本、样式、source map、图片、图标、字体和 wasm。
- 增加配置开关，默认启用；关闭后恢复现有 access log 行为。
- 不做前端请求分类或运行时配置方案。
- 只借鉴参考项目的需求意图，不照搬实现。

## Risk

- 如果扩展名集合仍未覆盖某些实际构建产物，仍会留下部分高频静态资源 `200` 日志。
- 如果扩展名集合过宽，可能隐藏本应保留的非典型 `/assets/` 成功请求日志。
- 由于过滤依赖最终 status，候选资产路径的 `request started` 必须延迟到 handler 返回之后；若 status 为 `200` 则整体跳过，否则补打 started/completed。

## User review notes

- 用户指出本需求没有那么复杂：路径 `/assets/` 开头、以 `js`、`css`、`jpg` 之类静态资源扩展名结尾的 `200` 请求过滤掉。
- 用户确认扩展名集合包含常见资源，并要求添加开关，默认启用。
