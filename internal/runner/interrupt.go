package runner

// Interrupt sequencing lives in runner.go so the state machine stays visible
// beside the wait/cleanup loop. This file intentionally remains small until
// M5 hardening adds richer interrupt strategies.
