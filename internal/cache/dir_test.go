package cache

import (
	"os"
	"testing"

	rtest "github.com/rubiojr/rapi/internal/test"
)

// DefaultDir should honor RESTIC_CACHE_DIR on all platforms.
func TestCacheDirEnv(t *testing.T) {
	cachedir := os.Getenv("RESTIC_CACHE_DIR")

	if cachedir == "" {
		cachedir = "/doesnt/exist"
		err := os.Setenv("RESTIC_CACHE_DIR", cachedir)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Unsetenv("RESTIC_CACHE_DIR")
	}

	dir, err := DefaultDir()
	rtest.Equals(t, cachedir, dir)
	rtest.OK(t, err)
}
