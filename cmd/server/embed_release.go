//go:build release

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var embeddedStatic embed.FS

func getStaticFS() fs.FS {
	sub, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		panic("failed to create sub filesystem: " + err.Error())
	}
	return sub
}
