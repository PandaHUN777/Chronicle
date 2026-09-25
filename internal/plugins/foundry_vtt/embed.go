package foundry_vtt

import "embed"

// MigrationsFS embeds the foundry_vtt plugin's migrations directory.
// Registered with the database.RunPluginMigrations runner from
// cmd/server/main.go.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
