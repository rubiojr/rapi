/*
 * Restic snapshot related tools
 *
 * See https://github.com/rubiojr/rapi/tree/master/docs/tooling
 */
package main

import (
	"github.com/urfave/cli/v2"
)

func init() {
	cmd := &cli.Command{
		Name:  "snapshot",
		Usage: "Snapshot operations",
		Subcommands: []*cli.Command{
			&cli.Command{
				Name:   "info",
				Action: printSnapshotInfo,
				Flags:  []cli.Flag{},
				Before: func(c *cli.Context) error {
					return setupApp(c)
				},
			},
		},
	}
	appCommands = append(appCommands, cmd)
}
