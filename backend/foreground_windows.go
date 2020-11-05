package backend

import (
	"os/exec"

	"github.com/rubiojr/rapi/internal/errors"
)

func startForeground(cmd *exec.Cmd) (bg func() error, err error) {
	// just start the process and hope for the best
	err = cmd.Start()
	if err != nil {
		return nil, errors.Wrap(err, "cmd.Start")
	}

	bg = func() error { return nil }
	return bg, nil
}
