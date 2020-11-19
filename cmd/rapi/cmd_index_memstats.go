package main

import (
	"context"
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/prometheus/procfs"
	"github.com/rubiojr/rapi"
	"github.com/urfave/cli/v2"
)

func init() {
	cmd := &cli.Command{
		Name:   "index-mem-stats",
		Usage:  "Print index memory consumption",
		Action: runIndexMemStats,
		Flags:  []cli.Flag{},
		Before: func(c *cli.Context) error {
			return setupApp(c)
		},
	}
	appCommands = append(appCommands, cmd)
}

func runIndexMemStats(c *cli.Context) error {
	p, err := procfs.Self()
	if err != nil {
		return err
	}

	before, err := p.NewStat()
	if err != nil {
		return err
	}

	rapiRepo, err = rapi.OpenRepository(globalOptions)
	tstart := time.Now()
	rapiRepo.LoadIndex(context.Background())
	tdiff := time.Since(tstart)

	after, err := p.NewStat()
	if err != nil {
		return err
	}

	fmt.Println("")

	fmt.Println("  Pre-load")
	printRow("cpu time", fmt.Sprintf("%.2f", before.CPUTime()), headerColor)
	printRow("vsize", humanize.Bytes(uint64(before.VirtualMemory())), headerColor)
	printRow("rss", humanize.Bytes(uint64(before.ResidentMemory())), headerColor)

	fmt.Println()

	fmt.Println("  Post-load")
	printRow("cpu time", fmt.Sprintf("%.2f", after.CPUTime()), headerColor)
	printRow("vsize", humanize.Bytes(uint64(after.VirtualMemory())), headerColor)
	printRow("rss", humanize.Bytes(uint64(after.ResidentMemory())), headerColor)

	fmt.Printf("\nIndex load time: %d ms\n", tdiff/time.Millisecond)

	return nil
}
