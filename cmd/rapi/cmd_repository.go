package main

import (
	"fmt"

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
	config := rapiRepo.Config()
	//fmt.Printf("%s %s\n", padding.String(colHeader("ID:"), colPadding), config.ID)
	printRow("ID", config.ID, headerColor)
	printRow("Location", rapiRepo.Backend().Location(), headerColor)
	printRow("Chunker polynomial", config.ChunkerPolynomial.String(), headerColor)
	printRow("Repository version", fmt.Sprintf("%d", config.Version), headerColor)
	return nil
}
