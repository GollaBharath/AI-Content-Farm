package httpserver

import "embed"

//go:embed web/*
var uiFS embed.FS
