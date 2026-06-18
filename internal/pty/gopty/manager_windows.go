package gopty

// Windows-specific cleanup hardening belongs in this adapter. M2 currently
// relies on ConPTY close plus Process.Kill fallback; richer process-tree
// cleanup is intentionally left for M5 hardening.
