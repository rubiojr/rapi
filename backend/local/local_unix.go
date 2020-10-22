// +build !windows

package local

import (
	"os"

	"github.com/rubiojr/rapi/internal/fs"
)

// set file to readonly
func setFileReadonly(f string, mode os.FileMode) error {
	return fs.Chmod(f, mode&^0222)
}
