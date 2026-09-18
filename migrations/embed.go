// Package migrations holds the versioned SQL migrations, embedded into the
// binary so the service can apply them at startup without shipping files.
package migrations

import "embed"

//go:embed *.sql
var files embed.FS

// FS exposes the embedded migration files as a read-only filesystem.
var FS = files
