package web

import "embed"

//go:embed all:out
var StaticFS embed.FS
