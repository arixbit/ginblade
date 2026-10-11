// Package locales embeds the API's message catalogs, one <locale>.json per language.
package locales

import "embed"

// FS holds every catalog in this directory, compiled into the binary.
//
//go:embed *.json
var FS embed.FS
