// Package web встраивает собранный фронт (web/dist) в бинарник.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist - содержимое web/dist; пустая папка, если фронт ещё не собран (тогда отдаётся только API).
func Dist() fs.FS {
	sub, _ := fs.Sub(dist, "dist")
	return sub
}
