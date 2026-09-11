package migrations

import "embed"

//go:embed sqlite/*.sql postgres/*.sql
var migrations embed.FS

func Embed() embed.FS {
	return migrations
}
