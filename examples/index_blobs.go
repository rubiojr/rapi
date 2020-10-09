/*
  index_blobs.go walks restic's repository data dir and adds all the blobs available
  the pack files found to an index map.
*/
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rubiojr/rapi/blob"
	"github.com/rubiojr/rapi/key"
	"github.com/rubiojr/rapi/pack"
	"github.com/rubiojr/rapi/restic"

	"github.com/rubiojr/rapi/examples/util"
)

var blobIndex = map[restic.ID]*blob.Blob{}

func main() {
	k := util.FindAndOpenKey()
	// Use a map to index all the blobs so we can easily find
	// which pack contains them later
	indexBlobs(util.RepoPath, k)
	fmt.Printf("%d blobs in the repository\n", len(blobIndex))
}

// walk restic's repository data dir and index all the pack files found
func indexBlobs(repoPath string, k *key.Key) {
	dataDir := filepath.Join(repoPath, "data")
	indexerFunc := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		_, packStr := filepath.Split(path)
		indexBlobsInPack(packStr, path, info, k)
		return nil
	}
	err := filepath.Walk(dataDir, indexerFunc)
	util.CheckErr(err)

}

// Add all the blob IDs found in a pack to the index map
func indexBlobsInPack(packID, path string, info os.FileInfo, k *key.Key) {
	handle, err := os.Open(path)
	util.CheckErr(err)
	defer handle.Close()
	blobs, err := pack.List(k.Master, handle, info.Size())

	for _, blob := range blobs {
		fmt.Printf("%s %s\n", blob.Type, blob.ID)
		pid, err := restic.ParseID(packID)
		util.CheckErr(err)
		blob.PackID = pid
		blobIndex[blob.ID] = &blob
	}
}
