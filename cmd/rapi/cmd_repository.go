package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/briandowns/spinner"
	"github.com/rubiojr/rapi/repository"
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
				Before: func(c *cli.Context) error {
					return setupApp(c)
				},
			},
			&cli.Command{
				Name:   "info",
				Action: printInfo,
				Flags:  []cli.Flag{},
				Before: func(c *cli.Context) error {
					return setupApp(c)
				},
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
	packCount, countErr := countPacks(rapiRepo)
	rapiRepo.LoadIndex(context.Background())
	s.Stop()

	printRow("ID", config.ID, headerColor)
	printRow("Location", rapiRepo.Backend().Location(), headerColor)
	printRow("Chunker polynomial", config.ChunkerPolynomial.String(), headerColor)
	printRow("Repository version", fmt.Sprintf("%d", config.Version), headerColor)
	if countErr != nil {
		printRow("Packs", "error", headerColor)
	} else {
		printRow("Packs", fmt.Sprintf("%d", packCount), headerColor)
	}
	printRow("Tree blobs", fmt.Sprintf("%d", rapiRepo.Index().Count(restic.TreeBlob)), headerColor)
	printRow("Data blobs", fmt.Sprintf("%d", rapiRepo.Index().Count(restic.DataBlob)), headerColor)
	return nil
}

func countPacks(repo *repository.Repository) (uint64, error) {
	var packCount uint64
	err := rapiRepo.List(context.Background(), restic.PackFile, func(id restic.ID, packSize int64) error {
		atomic.AddUint64(&packCount, 1)
		return nil
	})

	return packCount, err
}
