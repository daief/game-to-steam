// Package assets contains the application's shared visual assets.
package assets

import _ "embed"

//go:generate go run ../cmd/icon-gen

//go:embed icon.png
var Icon []byte
