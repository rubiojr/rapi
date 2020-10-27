package main

import (
	"os"

	"github.com/rubiojr/rapi"
	"github.com/rubiojr/rapi/repository"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var appCommands []*cli.Command
var repoPath string
var rapiRepo *repository.Repository
var globalOptions = rapi.DefaultOptions

func main() {
	var err error
	app := &cli.App{
		Name:     "rapi",
		Commands: []*cli.Command{},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "repo",
				Aliases:     []string{"r"},
				EnvVars:     []string{"RESTIC_REPOSITORY"},
				Usage:       "Repository path",
				Required:    false,
				Destination: &repoPath,
			},
			&cli.StringFlag{
				Name:        "password",
				Aliases:     []string{"p"},
				EnvVars:     []string{"RESTIC_PASSWORD"},
				Usage:       "Repository password",
				Required:    false,
				Destination: &globalOptions.Password,
			},
			&cli.BoolFlag{
				Name:     "debug",
				Aliases:  []string{"d"},
				Usage:    "Enable debugging",
				Required: false,
			},
		},
	}

	app.Commands = append(app.Commands, appCommands...)
	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func setupApp(ctx *cli.Context) error {
	var err error
	if ctx.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}
	globalOptions.Repo = repoPath
	rapiRepo, err = rapi.OpenRepository(globalOptions)
	return err
}
