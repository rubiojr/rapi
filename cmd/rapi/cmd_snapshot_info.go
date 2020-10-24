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
	"time"

	"github.com/briandowns/spinner"
	"github.com/dustin/go-humanize"
	"github.com/rubiojr/rapi/internal/walker"
	"github.com/rubiojr/rapi/restic"
	"github.com/urfave/cli/v2"
)

func printSnapshotInfo(c *cli.Context) error {
	var err error
	ctx := context.Background()

	if err = rapiRepo.LoadIndex(ctx); err != nil {
		return err
	}

	// create a container for the stats (and other needed state)
	stats := &statsContainer{
		uniqueFiles:  make(map[fileID]struct{}),
		uniqueInodes: make(map[uint64]struct{}),
		blobs:        restic.NewBlobSet(),
	}

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Color("fgHiRed")
	s.Suffix = " Calculating snapshot stats, this may take some time"
	s.Start()

	sid, err := restic.FindLatestSnapshot(ctx, rapiRepo, []string{}, []restic.TagList{}, []string{})
	if err != nil {
		return err
	}
	sn, err := restic.LoadSnapshot(ctx, rapiRepo, sid)
	if err != nil {
		return err
	}

	err = statsWalkSnapshot(ctx, sn, rapiRepo, stats)
	if err != nil {
		return fmt.Errorf("error walking snapshot: %v", err)
	}

	// the blob handles have been collected, but not yet counted
	for blobHandle := range stats.blobs {
		blobSize, found := rapiRepo.LookupBlobSize(blobHandle.ID, blobHandle.Type)
		if !found {
			return fmt.Errorf("blob %v not found", blobHandle)
		}
		stats.TotalBlobSize += uint64(blobSize)
		stats.TotalBlobCount++
	}

	s.Stop()
	printRow("Total Blob Count", fmt.Sprintf("%d", stats.TotalBlobCount), headerColor)
	printRow(
		"Unique Files Size",
		humanize.Bytes(stats.TotalBlobSize)+fmt.Sprintf(" (deduped %s)", humanize.Bytes(stats.RestoreSize-stats.TotalBlobSize)),
		headerColor,
	)
	printRow("Total Files", fmt.Sprintf("%d", stats.TotalFileCount), headerColor)
	printRow("Unique Files", fmt.Sprintf("%d", stats.UniqueFileCount), headerColor)
	printRow("Restore Size", humanize.Bytes(stats.RestoreSize), headerColor)

	return nil
}

func statsWalkSnapshot(ctx context.Context, snapshot *restic.Snapshot, repo restic.Repository, stats *statsContainer) error {
	if snapshot.Tree == nil {
		return fmt.Errorf("snapshot %s has nil tree", snapshot.ID().Str())
	}

	// count just the sizes of unique blobs; we don't need to walk the tree
	// ourselves in this case, since a nifty function does it for us
	restic.FindUsedBlobs(ctx, repo, *snapshot.Tree, stats.blobs)

	err := walker.Walk(ctx, repo, *snapshot.Tree, restic.NewIDSet(), statsWalkTree(repo, stats))
	if err != nil {
		return fmt.Errorf("walking tree %s: %v", *snapshot.Tree, err)
	}

	return nil
}

func statsWalkTree(repo restic.Repository, stats *statsContainer) walker.WalkFunc {
	return func(parentTreeID restic.ID, npath string, node *restic.Node, nodeErr error) (bool, error) {
		if nodeErr != nil {
			return true, nodeErr
		}
		if node == nil {
			return true, nil
		}

		// only count this file if we haven't visited it before
		fid := makeFileIDByContents(node)
		if _, ok := stats.uniqueFiles[fid]; !ok {
			// mark the file as visited
			stats.uniqueFiles[fid] = struct{}{}

			// simply count the size of each unique file (unique by contents only)
			stats.TotalSize += node.Size
			stats.UniqueFileCount++
		}

		stats.TotalFileCount++

		// if inodes are present, only count each inode once
		// (hard links do not increase restore size)
		if _, ok := stats.uniqueInodes[node.Inode]; !ok || node.Inode == 0 {
			stats.uniqueInodes[node.Inode] = struct{}{}
			stats.RestoreSize += node.Size
		}

		return true, nil
	}
}

// statsContainer holds information during a walk of a repository
// to collect information about it, as well as state needed
// for a successful and efficient walk.
type statsContainer struct {
	TotalSize       uint64
	RestoreSize     uint64
	TotalFileCount  uint64
	UniqueFileCount uint64
	TotalBlobCount  uint64
	TotalBlobSize   uint64

	// uniqueFiles marks visited files according to their
	// contents (hashed sequence of content blob IDs)
	uniqueFiles map[fileID]struct{}

	// uniqueInodes marks visited files according to their
	// inode # (hashed sequence of inode numbers)
	uniqueInodes map[uint64]struct{}

	// blobs is used to count individual unique blobs,
	// independent of references to files
	blobs restic.BlobSet
}

const (
	countModeRestoreSize           = "restore-size"
	countModeUniqueFilesByContents = "files-by-contents"
	countModeBlobsPerFile          = "blobs-per-file"
	countModeRawData               = "raw-data"
)
