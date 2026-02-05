package web

import "embed"

//go:embed all:*
var StaticFiles embed.FS
