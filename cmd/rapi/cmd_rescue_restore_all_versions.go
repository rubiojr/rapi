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
	"github.com/rubiojr/rapi/internal/filter"
	"github.com/rubiojr/rapi/internal/walker"
	"github.com/rubiojr/rapi/restic"
	"github.com/urfave/cli/v2"
)

type fileInfo struct {
	blobIDs restic.IDs
	path    string
	name    string
}

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

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Color("fgHiRed")
	s.Suffix = " Finding files to restore..."
	s.Start()

	if err = rapiRepo.LoadIndex(ctx); err != nil {
		return err
	}

	snapshots, err := restic.LoadAllSnapshots(ctx, rapiRepo, nil)
	if err != nil {
		return err
	}

	filesFound := map[fileID]fileInfo{}
	for _, sn := range snapshots {
		if sn.Tree == nil {
			return fmt.Errorf("snapshot %s has nil tree", sn.ID().Str())
		}
		err = walker.Walk(ctx, rapiRepo, *sn.Tree, restic.NewIDSet(), rescueWalkTree(pattern, filesFound))
		if err != nil {
			return fmt.Errorf("walking tree %s: %v", *sn.Tree, err)
		}
	}

	s.Stop()
	fmt.Printf("%d files matched. Restoring to target directory '%s'...\n", len(filesFound), targetDir)

	for k, v := range filesFound {
		hash := fmt.Sprintf("%x", k)
		dest := filepath.Join(targetDir, v.name+"_"+hash)
		f, err := os.Create(dest)
		if err != nil {
			panic(err)
		}
		for _, rid := range v.blobIDs {
			buf, err := rapiRepo.LoadBlob(ctx, restic.DataBlob, rid, nil)
			if err != nil {
				return err
			}
			f.Write(buf)
		}
		f.Close()
		destShort := filepath.Join(targetDir, v.name+"_"+hash[:5]+"...")
		fmt.Printf("** restored %s to %s\n", v.path, destShort)
	}

	return nil
}

func rescueWalkTree(pattern string, filesFound map[fileID]fileInfo) walker.WalkFunc {
	return func(parentTreeID restic.ID, npath string, node *restic.Node, nodeErr error) (bool, error) {
		if nodeErr != nil {
			return true, nodeErr
		}

		if node == nil {
			return true, nil
		}

		if node.Type != "file" {
			return true, nil
		}

		fid := makeFileIDByContents(node)
		if _, ok := filesFound[fid]; !ok {
			if ok, _ := filter.Match(pattern, npath); ok {
				meta := fileInfo{blobIDs: node.Content, path: npath, name: node.Name}
				filesFound[fid] = meta
			}
		}

		return true, nil
	}
}
