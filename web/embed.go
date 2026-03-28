package web

import "embed"

// DistFS contains the compiled frontend assets served by the Go app.
//
//go:embed dist/*
var DistFS embed.FS
