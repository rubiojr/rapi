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
		Name:  "rescue",
		Usage: "Snapshot operations",
		Subcommands: []*cli.Command{
			&cli.Command{
				Usage:  "Restore all files matching a pattern",
				Name:   "restore-all-versions",
				Action: restoreAllVersions,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "target",
						Usage:    "Directory where the files will be restored",
						Required: true,
					},
				},
				Before: func(c *cli.Context) error {
					return setupApp(c)
				},
			},
		},
	}
	appCommands = append(appCommands, cmd)
}
