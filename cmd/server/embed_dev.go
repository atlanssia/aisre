//go:build !release

package main

import (
	"io/fs"
	"os"
)

func getStaticFS() fs.FS {
	return os.DirFS("web/dist")
}
