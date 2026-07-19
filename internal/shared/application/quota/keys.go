package quota

import "errors"

const (
	KeyConcurrentAttaches     = "terminal.concurrent_attaches"
	CodeAttachExceeded        = "terminal_attach_quota_exceeded"
	MessageAttachExceeded     = "Terminal attach quota exceeded."
	DefaultConcurrentAttaches = 8
	LocalSubject              = "local"
)

var ErrExceeded = errors.New(CodeAttachExceeded)
