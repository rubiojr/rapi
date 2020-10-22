package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dustin/go-humanize"
	"github.com/rubiojr/rapi/crypto"
	"github.com/rubiojr/rapi/examples/util"
	"github.com/rubiojr/rapi/pack"
)

func main() {
	k := util.FindAndOpenKey()

	dataDir := filepath.Join(util.RepoPath, "data")

	packWalker := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		listPackBlobs(path, info, k)
		return nil
	}

	err := filepath.Walk(dataDir, packWalker)
	util.CheckErr(err)
}

// List pack blobs and some of their attributes
func listPackBlobs(path string, info os.FileInfo, k *crypto.Key) {
	handle, err := os.Open(path)
	util.CheckErr(err)
	blobs, err := pack.List(k, handle, info.Size())
	util.CheckErr(err)

	fmt.Println("Pack file: ", path)
	fmt.Println("  Size: ", humanize.Bytes(uint64(info.Size())))
	fmt.Println("  Blobs: ", len(blobs))
	for _, blob := range blobs {
		// Describe blob and print content
		fmt.Printf(
			"    %s: %s (%s)\n",
			blob.Type.String(),
			blob.ID.String(),
			humanize.Bytes(uint64(blob.Length)),
		)
	}
}
