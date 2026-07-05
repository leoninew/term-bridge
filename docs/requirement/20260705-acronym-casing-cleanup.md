# Acronym Casing Cleanup

最后修改时间: 2026-07-05 22:15:00

## Review status

Accepted

## Background

项目 Go 源码里缩写词大小写不统一：有些用全大写（`API`、`ID`、`URL`、`DB`、`JWT`、`CORS`），有些用半大写（`Api`、`Id`、`Url`、`Db`、`Jwt`、`Cors`）。已经在新 CLAUDE.md 里定下规则：

> 缩写词在 PascalCase / CamelCase 中只用首字母大写：`Api`、`Id`、`Url`、`Uri`、`Db`。
> 禁止 `API`、`ID`、`URL`、`URI`、`DB` 这种全大写形式。

现在需要把存量代码按规则统一。

## Goal

- Go 源码里所有属于"项目内部缩写"的标识符，统一为首字母大写的半大写形式。
- 规则仅对 Go 源文件生效（`internal/`、`cmd/`）。前端暂不处理。
- 分布式的 `.proto` 源文件同步改为 `Id`、`Url`、`Uri`、`Api`、`Db` 等形式（以便未来重新生成）。
- 已经存在的 `.pb.go` 生成文件**不在本次范围**（生成物，手工改无意义）。

## Non-goal

- 不处理 `.pb.go` 生成文件（protobuf 生成物）。
- 不处理 Web 前端（`web/src/`）。
- 不处理标准库引用的全大写标识符（`r.URL.*`、`http.ErrServerClosed`、`base64.URLEncoding` 等）。
- 不改 JSON tag（JSON tag 是 API 契约，独立于 Go 字段名，不受本规则影响）。
- 不改公开 API 兼容性——纯大小写调整在 Go 1.x 里属于重命名，需要一次性改完所有调用方。
- 不改 RFC/协议相关的全大写缩写名（见下面的"保留全大写"清单）。

## Acronyms 处理清单

### A. 必须改成半大写（核心四个 + 附带五个，全部处理）

| 现用 | 改为 | 大致行数 | 主要位置 |
|---|---|---|---|
| `APIBaseURL` | `ApiBaseUrl` | ~8 | `internal/shared/infrastructure/config/`、`internal/{agent,cloud}/application/bootstrap/` |
| `APIKey` | `ApiKey` | ~5 | `internal/shared/infrastructure/config/`、`internal/cloud/application/bootstrap/` |
| `GateAPIConfig` | `GateApiConfig` | ~2 | `internal/{agent,cloud}/api/handler/server.go` |
| `APIErrorResp[T]` | `ApiErrorResp[T]` | ~1 | `internal/cloud/api/handler/dto.go` |
| `RedirectURL` | `RedirectUrl` | ~3 | `internal/{agent,cloud}/application/bootstrap/server.go` |
| `ID`（用户/设备/会话 ID 字段，非 proto 引用） | `Id` | ~40 | `internal/cloud/repository/user/auth/`、`internal/cloud/application/user/auth/service.go`、`internal/agent/repository/device/repository.go` |
| `database.DB` | `database.Db` | ~10 | `internal/shared/infrastructure/database/db.go` |
| `DBStore`（agent 数据库 struct） | `DbStore` | ~30+ | `internal/agent/repository/task/state/db_store.go` |
| `JWTTTL` / `JWTConfig` / `JWTSecret` | `JwtTTL` / `JwtConfig` / `JwtSecret` | ~30 | `internal/shared/infrastructure/config/` |
| `CORSAllowedOrigins` / `CORS` / `CORSForPaths` | `CorsAllowedOrigins` / `Cors` / `CorsForPaths` | ~40 | `internal/shared/infrastructure/config/`、`internal/shared/api/middleware/cors.go` |
| `DSN`（Data Source Name 字段） | `Dsn` | ~15 | config 结构 |
| `TTL`（Time To Live 字段） | `Ttl` | ~12 | auth code policy |
| `MessageID` | `MessageId` | ~1 | `internal/cloud/infrastructure/email/resend.go` |

### B. `.proto` 源文件同步修改

- 路径：`proto/termbridge/{common,cloud,runtime,tunnel}/v1/*.proto`
- 改动：`optional string id = N;` → `optional string id = N;`（proto 字段名本身用 snake_case，不动）
- 改动：`message User { string Id = 1; }` → `message User { string Id = 1; }`（.proto message 字段用 PascalCase，应改为半大写）
- 注：本任务先改 `.proto` 源、记录需要重新生成 `.pb.go` 的清单；`.pb.go` 的实际重新生成不计入本次交付范围。

### C. 保留全大写（RFC / 标准协议 / 标准库名，不是项目内部缩写）

| 缩写 | 原因 |
|---|---|
| `HTTP` / `HTTPS` | RFC 7230 协议名 |
| `JSON` | RFC 8259 格式名 |
| `YAML` | 格式名 |
| `SQL` | 查询语言名 |
| `OAuth` | IETF RFC 6749 协议名（类名 `OAuthConfig` 仍保持全大写） |
| `URI` | RFC 3986 标准名（本字段使用半大写 `Uri` 仅在 URL 字段出现，但 HTTP URI 标准引用保留全大写） |
| `EOF` | Go `io.EOF` 错误 |
| `PTY` | Unix 伪终端名（POSIX 标准概念） |
| `SSH` | 协议名 |
| `TLS` | 协议名 |
| `UTC` | Go `time.UTC` 时区名 |
| `RPC` | 协议名 |
| `UUID` | RFC 4122 标准名 |
| `JWT` 作为 RFC 7519 原名时 | 正式名称用 JWT，但 Go 字段名改为 `Jwt` |
| `CORS` 作为 HTTP 机制原名时 | 正式名称用 CORS，但 Go 字段名改为 `Cors` |

## Decisions（已确认）

1. **全部 9 组缩写一起处理**（API/URL/ID/DB/JWT/CORS/DSN/TTL/MessageID），不拆分多次 PR。
2. **`.proto` 源文件也要同步改**：`Id`、`Url`、`Uri`、`Api` 等 PascalCase 字段名改为半大写，保证未来重新生成 `.pb.go` 时自动对齐。
3. **已有的 `.pb.go` 生成文件不在本次范围**（不手工改生成物）。
4. **`DB` 结构体（`database.DB`）和 `DBStore`（agent 数据库 struct）**统一改，接受大规模 diff。
5. **保留全大写清单**写入 CLAUDE.md 缩写大小写章节。

## Risk

- **`DBStore`** 是 agent 侧的公开 struct，约 30+ 个方法签名包含它，改全名会触发大规模 diff。需要配合 `gopls rename` 做语义级重命名。
- **`database.DB`** 被 `agent` 和 `cloud` 两侧同时使用，是跨 bounded context 的类型，需要同步改名。
- **proto 生成 `.pb.go` 文件** 不在本次范围，但新业务代码如果引用它们的 `ID` 字段，仍会出现 mixed style——需要在 proto 重新生成后，通过静态检查强制收敛。
- **`model.UserView{ID: ...}` 等构造** 里的 `ID` 是 Go 字段名，改名要联动所有构造点和读取点。

## Acceptance

- [x] 规则已写入项目 `CLAUDE.md`（完成）。
- [x] 范围决策已确认（9 组缩写 + .proto 源文件，不含 .pb.go）。
- [ ] `internal/`、`cmd/` 所有 Go 源文件里 `APIBaseURL`、`APIKey`、`APIErrorResp`、`GateAPIConfig`、`RedirectURL` 已改为半大写。
- [ ] `internal/`、`cmd/` 所有 Go 源文件里公开 struct 字段名 `ID` 在非 proto 生成的代码中已改为 `Id`。
- [ ] `database.DB` → `database.Db` 完成。
- [ ] `DBStore` → `DbStore` 完成。
- [ ] `JWT*` → `Jwt*`、`CORS*` → `Cors*`、`DSN` → `Dsn`、`TTL` → `Ttl`、`MessageID` → `MessageId` 完成。
- [ ] `.proto` 源文件里的 PascalCase 字段名改为半大写。
- [ ] `go build ./...` 编译通过。
- [ ] `go test ./...` 测试通过。
- [ ] 保留全大写清单已在 CLAUDE.md 记录。

## User review notes

- 决策 1（全 9 组一起处理）—— **已确认**。
- 决策 2（.proto 源文件同步改）—— **已确认**。
- 决策 3（已有 .pb.go 不在范围）—— **已确认**。
- 决策 4（DB 系列全改）—— **已确认**。
- 决策 5（保留清单入 CLAUDE.md）—— **已确认**。
