package migrations

import "embed"

//go:embed sqlite/*.sql mysql/*.sql
var FS embed.FS
