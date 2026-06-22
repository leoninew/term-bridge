# Frontend Error Handling Guidelines

前端异常处理应遵循分层、分类、统一出口、上下文展示的原则。目标是让用户看到清晰、可操作的提示，同时避免技术错误污染页面布局或在多个位置重复出现。

## Core Principles

1. **Classify errors before displaying them**
   - 不同类型的错误应使用不同的展示方式。
   - 不要把所有异常都统一塞进页面顶部 banner 或全局状态。

2. **Use one primary user-facing surface per error**
   - 同一个错误不要同时出现在 inline error、toast、dialog 和 banner 中。
   - 如果需要记录技术详情，应进入日志、监控或开发者工具，而不是重复展示给用户。

3. **Keep API utilities UI-agnostic**
   - API 层负责请求、解析响应、规范化并抛出错误。
   - API 层不应直接调用 toast、dialog 或组件状态。
   - UI 层根据当前上下文决定错误如何展示。

4. **Prefer contextual error display**
   - 错误发生在哪里，就尽量在对应上下文中展示。
   - 没有明确上下文、非阻塞的操作失败，可以使用 toast。

5. **Avoid leaking unnecessary technical details**
   - 用户提示应清晰、简短、可理解。
   - HTTP 状态码、接口路径、堆栈、request id 等技术信息应优先进入日志或监控。

## Recommended Error Surfaces

| Error Type | Examples | Recommended Surface |
| --- | --- | --- |
| User input validation | Required field, invalid format | Field-level inline error |
| Form-level validation | Cross-field validation, submit precondition failed | Form-level inline error |
| Local section load failure | A panel, tab, widget, or list failed to load | Section-level inline error |
| Non-blocking API action failure | Save, delete, refresh, reorder, background sync | Toast |
| Page initialization failure | Page cannot function without required data | Page-level error state with retry |
| Destructive action confirmation | Delete, remove, reset | Dialog before action; toast or inline result after action |
| Runtime rendering failure | Unexpected component crash | Error boundary / fallback view |
| Connectivity issue | WebSocket disconnect, network unavailable | Contextual inline state and/or toast depending on scope |

## Toast Usage

Toast is appropriate for transient, non-blocking operation results.

Use toast for:

- Save/create/update/delete failures.
- Background refresh failures.
- Reorder or sync failures.
- Operation success messages.
- Informational messages that do not require layout changes.

Avoid toast as the only surface for:

- Field validation errors.
- Errors that block an entire page from working.
- Errors requiring detailed user action.
- Long-lived state that users need to inspect while continuing work.

Recommended pattern:

```ts
try {
  await performAction()
  pushToast('success', title, description)
} catch (err) {
  notifyError(failureTitle, err)
}
```

## Inline Error Usage

Inline errors are appropriate when the error is tied to a specific user input or UI region.

Use inline errors for:

- Form fields.
- Form submit validation.
- A failed list/panel/tab/widget load.
- A terminal/session/connection area that has its own lifecycle.

Recommended pattern:

```ts
sectionError.value = null
sectionLoading.value = true
try {
  data.value = await loadSectionData()
} catch (err) {
  sectionError.value = userFacingErrorMessage(err)
} finally {
  sectionLoading.value = false
}
```

A local inline error may be paired with a toast when the action is important and the toast helps the user notice the failure, but avoid duplicating the exact same message in multiple prominent places unless there is a clear reason.

## Page-Level Error State

Use a page-level error state only when the page cannot provide its primary function without the failed data or initialization step.

A page-level error state should usually include:

- A short explanation.
- A retry action.
- Optional navigation or fallback action.

Example:

```txt
Unable to load this page.
[Retry]
```

Do not use a page-level banner for routine API action failures if the page remains usable.

## Dialog Usage

Dialogs should generally be used for user decisions, not passive error reporting.

Use dialogs for:

- Confirming destructive actions.
- Asking the user to choose between recovery options.
- Showing blocking issues that require explicit acknowledgement.

Avoid dialogs for:

- Routine API failures.
- Background refresh failures.
- Errors that can be handled with inline state or toast.

## Error Normalization

Centralize conversion from unknown thrown values into a displayable message.

Simple projects can use:

```ts
function errorMessage(err: unknown) {
  return err instanceof Error ? err.message : String(err)
}

function notifyError(title: string, err: unknown) {
  pushToast('error', title, errorMessage(err))
}
```

Larger projects should prefer normalized application errors:

```ts
type AppError = {
  message: string
  code?: string
  status?: number
  details?: unknown
}
```

The API layer can normalize transport errors into `AppError`, while UI code decides the presentation.

## API Layer Responsibilities

API utilities should:

- Build requests.
- Parse responses.
- Normalize failed responses.
- Throw typed or structured errors.

API utilities should not:

- Show toast notifications.
- Mutate component state.
- Open dialogs.
- Decide page-level UI behavior.

Recommended shape:

```ts
export async function requestData() {
  const response = await fetch('/api/example')
  if (!response.ok) {
    throw new ApiError(response.status, 'Request failed')
  }
  return response.json()
}
```

UI code:

```ts
try {
  data.value = await requestData()
} catch (err) {
  notifyError('Failed to load data', err)
}
```

## Loading and Recovery

Always clear or set loading state predictably:

```ts
loading.value = true
try {
  data.value = await loadData()
} catch (err) {
  notifyError('Failed to load data', err)
} finally {
  loading.value = false
}
```

When requests can overlap, protect against stale results:

```ts
let requestVersion = 0

async function refresh() {
  const version = ++requestVersion
  const next = await loadData()
  if (version !== requestVersion) {
    return
  }
  data.value = next
}
```

For cancellable operations, prefer `AbortController` where supported.

## Logging and Observability

User-facing messages should be concise. Diagnostic details should be captured separately.

Prefer logging:

- Raw error object.
- HTTP status.
- Endpoint or operation name.
- Request id or trace id.
- User action context.

Do not expose stack traces or internal implementation details to users by default.

## Checklist

Before adding an error display, verify:

- Is this a validation, local, page-level, or non-blocking operation error?
- Is there exactly one primary user-facing surface for this error?
- Does the message tell the user what happened in understandable language?
- Are technical details logged instead of overexposed?
- Is the API layer free of UI-specific behavior?
- Are loading states reset in `finally`?
- Are contextual inline errors preserved where they help recovery?
- Is the implementation scoped to the current need without adding unnecessary abstractions?
