// Embedded web contains the compiled React/Vite application output.
package assets

import "embed"

var (
	//go:embed templates
	Templates embed.FS

	//go:embed web
	Web embed.FS
)
