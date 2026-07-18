# 运行时错误安全暴露与领域错误目录（总册）

最后修改时间: 2026-07-18 20:12:35

Review status: Accepted

## Flow mode / Stage

轻量模式 / light；需求 / Requirement（Accepted）；实现 / Implementation 已完成，待用户进入验证。

本文档合并自：

- `20260718-safe-runtime-error-exposure.md`（旁路 bad_control、runtimeerr 接口、file/git/session attach/closed、disconnect 枚举；其本身已合并更早两份草稿）
- `20260718-session-p1-error-catalog.md`（会话 P1 高频业务错误）
- `20260718-workspace-shortcut-error-catalog.md`（工作区删除约束 + shortcut Usage）
- `20260718-session-p2-input-error-catalog.md`（输入校验与 Normalize）

## Background

评估 `expose_errors` 后，REST 主路径已由 `writeAPIError` / `responseErrorText` 控制对外文案，但存在：

1. **旁路泄漏**：终端 WebSocket `bad_control` 直接写 `err.Error()`；隧道 `runtimeErrorFrame` 只回 code、固定 `Runtime request failed.`，丢失业务安全 message。
2. **领域 code 缺失**：会话/工作区/快捷方式大量 `apperrors.Usage/NotFound/Config`，HTTP 仅笼统 `bad_request` / `not_found`，前端无法稳定分支。
3. **disconnect 自由字符串**：设备断连 message 直接推 reason。
4. **接口不统一**：仅 file 能携带业务 message，其它领域无法走同一隧道回填通道。

## Goal

### A. 旁路安全暴露

1. 终端 `bad_control`：浏览器固定安全文案；cause 仅日志。
2. 隧道 `runtimeErrorFrame`：经 `runtimeerr.Error` 回填 code + 安全 message；非领域错误保持 `Runtime request failed.`。
3. Agent / Cloud 终端路径对称。

### B. 统一运行时错误接口与基础 catalog

1. `internal/shared/common/runtimeerr.Error`：`RuntimeErrorCode` + `RuntimeErrorMessage`。
2. file / session / git / workspace / shortcut 实现该接口。
3. Git：`invalid_git_operation`。
4. HTTP `domainErrorResponse` 映射领域 code；`apperrors` Usage/NotFound/Config 保守兜底（Config 不暴露内部 cause）。
5. disconnect reason 枚举 + 浏览器安全文案。

### C. 会话 P1 高频业务错误

- `cannot_delete_running_session`
- `cannot_rerun_running_session` / `session_already_running`
- `session_not_editable`
- `missing_session_name` / `missing_session_command`
- `session_not_found` / `workspace_not_found`
- 既有：`session_not_attachable` / `session_closed`

### D. 工作区 / 快捷方式业务约束

- `cannot_delete_workspace_with_running_sessions`
- `shortcut_id_required` / `shortcut_not_found` / `shortcut_disabled`

### E. P2 输入校验

会话：command shape/source、size、cwd、update fields、order ids/duplicate  
工作区：workspace ids required/duplicate  
快捷方式 Normalize：name/command required

## Non-goal

- 不强制 production `expose_errors=false`（独立项）。
- P3 Runtime 内部失败（start pty / save state / list / archive 等）继续 `upstream_error` + 日志。
- 不改 Git CLI stderr 直出产品文案。
- 不改 session StateRecord 历史 reason 持久化语义。
- 不统一静态资源 / CORS 非 JSON 错误形态。
- 不把 REST `responseErrorText` 再抽象成跨协议公共中间件。
- `history archive name exhausted` / `unsupported home path` 等内部路径错误不进产品 catalog。

## User scenarios

1. 非法 terminal attach size / 非法 control JSON → `bad_control` 固定文案，日志有细节。
2. 经 Cloud 隧道文件错误 → 与本地一致的业务 code + message。
3. 会话不可 attach / 已关闭 / 运行中不可删改重跑 / 缺名缺命令 / 找不到 → 稳定 code + 安全文案。
4. 工作区含运行中会话不可删；快捷方式 id/未找到/禁用 → 稳定 code。
5. 非法命令形态、source、终端尺寸、cwd、排序 ids 重复/缺失、shortcut name/command 缺失 → 400 领域 code。
6. 设备断连/重连/删除 → 浏览器 disconnect 枚举文案。
7. 非领域内部失败 → `runtime_error` / `upstream_error` + 通用安全文案。

## Acceptance

### 旁路

- [x] `bad_control` 不再使用 `sizeErr.Error()` / DecodeClient `err.Error()` 作为浏览器 message。
- [x] 服务端日志仍记录原始错误；共享常量避免 Agent/Cloud 漂移。

### 接口与基础 catalog

- [x] `runtimeerr.Error` 含 Message；`runtimeErrorFrame` 走接口回填。
- [x] file/session/git（及后续 workspace/shortcut）实现接口。
- [x] `invalid_git_operation`；disconnect `BrowserDisconnectError`。
- [x] HTTP `domainErrorResponse` 覆盖领域 code；`precondition_required` 等 file 映射保留。

### 会话 P1

- [x] P1 code 构造函数与 registry 替换；HTTP/隧道可识别；单测覆盖。

### 工作区 / 快捷方式

- [x] workspace 删除约束 + shortcut id/not_found/disabled；映射与单测。

### P2 输入校验

- [x] 会话/工作区/shortcut Normalize 输入类 Usage/Config 改为领域 code。
- [x] registry 中已无 `apperrors.Usage` / `Config`（Runtime 内部失败保留）。
- [x] 相关 Go 单测通过。

## Decisions

1. light 模式分轮实现，最终合并为本总册。
2. 接口：`internal/shared/common/runtimeerr`。
3. 领域错误文件：
   - session：`internal/agent/model/task/session/errors.go`
   - workspace：`internal/agent/model/task/workspace/errors.go`
   - shortcut：`internal/agent/model/task/shortcut/errors.go`
   - git：`internal/agent/model/task/git/errors.go`
   - file：扩展 `RuntimeErrorMessage`
4. 终端/disconnect 常量：`internal/shared/dto/protocol/terminal`。
5. HTTP 映射：Agent/Cloud `domainErrorResponse`。
6. status 约定：输入类 400；缺失 404；运行中/禁用等冲突 409；未知领域 default → 502 upstream。
7. 隧道业务 message 只来自领域安全 message，不回传任意 `err.Error()`。

## 领域 code 一览（实现后）

### 终端 / disconnect

| code | 说明 |
| --- | --- |
| `bad_control` | 非法 attach size / control 消息 |
| `device_disconnected` | 设备断连类（message 按 reason 枚举） |

### Session

| code | HTTP |
| --- | --- |
| `session_not_attachable` | 404 |
| `session_closed` | 409 |
| `session_not_found` | 404 |
| `workspace_not_found` | 404 |
| `missing_session_name` | 400 |
| `missing_session_command` | 400 |
| `session_not_editable` | 409 |
| `session_already_running` | 409 |
| `cannot_rerun_running_session` | 409 |
| `cannot_delete_running_session` | 409 |
| `invalid_session_command_shape` | 400 |
| `invalid_session_command_source` | 400 |
| `session_command_source_requires_command` | 400 |
| `session_update_requires_fields` | 400 |
| `invalid_terminal_size` | 400 |
| `invalid_session_cwd` | 400 |
| `session_ids_required` | 400 |
| `duplicate_session_id` | 400 |

### Workspace

| code | HTTP |
| --- | --- |
| `cannot_delete_workspace_with_running_sessions` | 409 |
| `workspace_ids_required` | 400 |
| `duplicate_workspace_id` | 400 |

### Shortcut

| code | HTTP |
| --- | --- |
| `shortcut_id_required` | 400 |
| `shortcut_not_found` | 404 |
| `shortcut_disabled` | 409 |
| `shortcut_name_required` | 400 |
| `shortcut_command_required` | 400 |

### Git / File（本主题相关）

| code | HTTP |
| --- | --- |
| `invalid_git_operation` | 400 |
| 既有 file codes（`file_not_found`、`revision_conflict`、`precondition_required` 等） | 见 `domainErrorResponse` |

## Risk

- 前端若依赖旧自由文案 / `bad_request` 字符串匹配，需改为认稳定 code。
- P3 Runtime 仍为通用 upstream；需靠日志 + `request_id` 排障。
- 新增领域错误必须实现 `runtimeerr.Error` 并补 HTTP 映射，否则落 default 502。

## 实现记录

### 旁路与接口

- `terminalproto.ErrorCodeBadControl` / `ErrorMessageBadControl`；Agent/Cloud 终端路径固定文案。
- `runtimeerr.Error`；`runtimeErrorFrame` 统一接口回填。
- disconnect：`BrowserDisconnectError`；Cloud `closeTerminals` 浏览器安全文案。
- Git：`invalid_git_operation`。

### 会话 / 工作区 / 快捷方式 catalog

- session/workspace/shortcut `errors.go` 与 registry 调用点替换。
- shortcut `Normalize` → `NameRequired` / `CommandRequired`。
- Agent/Cloud `domainErrorResponse` 扩展映射。
- registry 中已无 `apperrors.Usage` / `Config`；Runtime 内部失败保留。

### 测试（代表）

- runtimeErrorFrame / runtimeErrorResponse / domainErrorResponse 领域映射。
- `TestBrowserDisconnectError`。
- registry shortcut/session 相关断言改为 code 校验。
