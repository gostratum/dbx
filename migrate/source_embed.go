package migrate

import (
	"embed"
)

// EmbeddedMigrations is a placeholder for applications to provide their own embedded migrations
// Applications should provide their own embed.FS via fx.Provide() or WithEmbedFS()
var EmbeddedMigrations embed.FS

// WithEmbeddedFS allows using a custom embedded filesystem
// This is useful for testing or when migrations are embedded in a different package
func WithEmbeddedFS(embedFS embed.FS, subdir string) Option {
	return func(c *Config) {
		c.UseEmbed = true
		c.Dir = ""
		c.EmbedFS = embedFS
		c.EmbedSubdir = subdir
	}
}
