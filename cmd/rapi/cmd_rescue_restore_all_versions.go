/*
 * A modified version of Restic's `stats` command, but for a given snapshot.
 *
 * See https://github.com/rubiojr/rapi/tree/master/docs/tooling
 *
 * Original source from https://github.com/restic/restic/blob/31b8d7a63999746623c8940f8200205a52a2b81b/cmd/restic/cmd_stats.go
 */
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/briandowns/spinner"
	"github.com/rubiojr/rapi/restic"
	"github.com/rubiojr/rapi/walker"
	"github.com/urfave/cli/v2"
)

var s = spinner.New(spinner.CharSets[11], 100*time.Millisecond)

func restoreAllVersions(c *cli.Context) error {
	pattern := c.Args().First()
	ctx := context.Background()
	targetDir := c.String("target")

	_, err := os.Stat(c.String("target"))
	if err != nil {
		return err
	}

	if pattern == "" {
		return fmt.Errorf("missing pattern argument")
	}

	s.Color("fgHiRed")
	s.Suffix = " Finding files to restore..."
	s.Start()

	if err = rapiRepo.LoadIndex(ctx); err != nil {
		return err
	}

	filesFound := map[fileID]bool{}
	restic.ForAllSnapshots(ctx, rapiRepo, nil, func(id restic.ID, sn *restic.Snapshot, err error) error {
		if err != nil {
			return err
		}

		if sn.Tree == nil {
			return fmt.Errorf("snapshot %s has nil tree", sn.ID().Str())
		}
		return walker.Walk(ctx, rapiRepo, *sn.Tree, restic.NewIDSet(), rescueWalkTree(pattern, filesFound, targetDir))
	})

	s.Stop()
	fmt.Printf("%d files matched.\n", len(filesFound))
	return nil
}

func restoreFile(fid fileID, name string, blobIDs restic.IDs, targetDir string) error {
	hash := fmt.Sprintf("%x", fid)
	fname := hash[:8] + "_" + name
	if len(fname) >= 254 {
		fname = "+" + fname[len(fname)-254:]
	}
	dest := filepath.Join(targetDir, fname)

	f, err := os.Create(dest)
	defer f.Close()
	if err != nil {
		panic(err)
	}

	for _, rid := range blobIDs {
		buf, err := rapiRepo.LoadBlob(context.Background(), restic.DataBlob, rid, nil)
		if err != nil {
			return err
		}
		f.Write(buf)
	}

	return nil
}

func rescueWalkTree(pattern string, filesFound map[fileID]bool, targetDir string) walker.WalkFunc {
	return func(parentTreeID restic.ID, npath string, node *restic.Node, nodeErr error) (bool, error) {
		if nodeErr != nil {
			return true, nodeErr
		}

		if node == nil || node.Type != "file" {
			return true, nil
		}

		fid := makeFileIDByContents(node)
		if _, ok := filesFound[fid]; ok {
			return true, nil
		}

		if ok, _ := filepath.Match(pattern, node.Name); ok {
			filesFound[fid] = true
			err := restoreFile(fid, node.Name, node.Content, targetDir)
			if err != nil {
				s.Suffix = fmt.Sprintf(" error %s", node.Name)
			} else {
				s.Suffix = fmt.Sprintf(" restored %s", node.Name)
			}
		}

		return true, nil
	}
}
