# CLAUDE.md

opher-first
Don't fucking change the code without a word
Don't fucking tolerate errors infinitely; assertions first

## 测试断言

- 断言必须表达业务语义，禁止把实现细节的自然结果当作验证目标（例如断言某张表存在、某个字段名为 xxx 等）。不存在的东西不可穷举，逐条写进去没有价值。
- 断言应该回答"做了什么"和"结果是什么"，聚焦业务动作的副作用。

## 配置、正则、校验

- 配置项、正则表达式、输入校验规则属于**配置阶段**的工作，不要散落在业务逻辑里。
- 业务代码里出现硬编码正则、内联校验逻辑、魔法配置值，都是配置阶段没做完的信号。先把它们集中到配置层（const / config struct / validator 注册器），再写业务。

## 类型优先

- 禁止使用字典嵌套作为数据结构，例如 `map[string]map[string]string{}`。两层以上的 map 或 map 套 map 一律视为类型缺失的信号。
- 有结构的数据必须定义为 struct（或 typed alias），让它携带字段名、类型约束和文档意图。"字典嵌套"是绕开类型检查的捷径，也是后续重构的债务。

## 缩写大小写

- 项目内部缩写在 PascalCase / CamelCase 中只用首字母大写：`Api`、`Id`、`Url`、`Uri`、`Db`、`Jwt`、`Cors`、`Dsn`、`Ttl`。
- 禁止对这些缩写使用全大写形式（`API`、`ID`、`URL`、`URI`、`DB`、`JWT`、`CORS`、`DSN`、`TTL`）。

标准 RFC / 协议 / 机制名保持全大写（这些是正式名称，不是项目内部缩写）：

- `HTTP` / `HTTPS` — RFC 7230
- `JSON` — RFC 8259
- `YAML`
- `SQL`
- `OAuth` — IETF RFC 6749
- `SSH` / `TLS`
- `RPC`
- `UUID` — RFC 4122
- `EOF` — Go `io.EOF`
- `PTY` — POSIX 伪终端
- `UTC` — Go `time.UTC`

标识符命名也是 API 契约的一部分，写错等于暴露不专业。
