package main

import (
	"context"
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/rubiojr/rapi/restic"
	"github.com/urfave/cli/v2"
)

func init() {
	cmd := &cli.Command{
		Name:  "repository",
		Usage: "Repository operations",
		Subcommands: []*cli.Command{
			&cli.Command{
				Name:   "id",
				Action: printID,
				Flags:  []cli.Flag{},
			},
			&cli.Command{
				Name:   "info",
				Action: printInfo,
				Flags:  []cli.Flag{},
			},
		},
	}
	appCommands = append(appCommands, cmd)
}

func printID(c *cli.Context) error {
	id := rapiRepo.Config().ID
	fmt.Println(id)
	return nil
}

func printInfo(c *cli.Context) error {

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Color("fgHiRed")
	s.Suffix = " Loading index, this may take some time"
	s.Start()

	config := rapiRepo.Config()
	rapiRepo.LoadIndex(context.Background())
	s.Stop()

	printRow("ID", config.ID, headerColor)
	printRow("Location", rapiRepo.Backend().Location(), headerColor)
	printRow("Chunker polynomial", config.ChunkerPolynomial.String(), headerColor)
	printRow("Repository version", fmt.Sprintf("%d", config.Version), headerColor)
	printRow("Packs", fmt.Sprintf("%d", len(rapiRepo.Index().Packs())), headerColor)
	printRow("Tree blobs", fmt.Sprintf("%d", rapiRepo.Index().Count(restic.TreeBlob)), headerColor)
	printRow("Data blobs", fmt.Sprintf("%d", rapiRepo.Index().Count(restic.DataBlob)), headerColor)
	return nil
}
