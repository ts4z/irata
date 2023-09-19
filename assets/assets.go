package assets

import (
	"embed"
)

//go:embed fs/*
var FS embed.FS

//go:embed special/*
var Special embed.FS

//go:embed templates/*
var Templates embed.FS
