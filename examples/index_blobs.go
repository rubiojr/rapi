/*
  index_blobs.go walks restic's repository data dir and adds all the blobs available
  the pack files found to an index map.
*/
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/rubiojr/rapi/blob"
	"github.com/rubiojr/rapi/key"
	"github.com/rubiojr/rapi/pack"
	"github.com/rubiojr/rapi/restic"

	"github.com/rubiojr/rapi/examples/util"
)

var blobIndex = map[restic.ID]*blob.Blob{}
var indexedPacks = 0
var indexedBlobs = 0
var totalPacks = 0

func main() {
	k := util.FindAndOpenKey()
	// Use a map to index all the blobs so we can easily find
	// which pack contains them later
	indexBlobs(util.RepoPath, k)
	fmt.Printf("%d blobs and %d packs found in the repository\n", len(blobIndex), totalPacks)
}

// walk restic's repository data dir and index all the pack files found
func indexBlobs(repoPath string, k *key.Key) {
	dataDir := filepath.Join(repoPath, "data")
	indexerFunc := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		indexBlobsInPack(info.Name(), path, info, k)
		return nil
	}
	go progressMonitor()
	err := filepath.Walk(dataDir, indexerFunc)
	util.CheckErr(err)
}

// Add all the blob IDs found in a pack to the index map
func indexBlobsInPack(packID, path string, info os.FileInfo, k *key.Key) {
	handle, err := os.Open(path)
	util.CheckErr(err)
	defer handle.Close()
	blobs, err := pack.List(k.Master, handle, info.Size())
	util.CheckErr(err)

	for _, blob := range blobs {
		pid, err := restic.ParseID(packID)
		util.CheckErr(err)
		blob.PackID = pid
		blobIndex[blob.ID] = &blob
		indexedBlobs += 1
	}
	indexedPacks += 1
}

func progressMonitor() {
	fmt.Println("Scanning data directory...")
	files, err := ioutil.ReadDir(filepath.Join(util.RepoPath, "data"))
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range files {
		packs, err := ioutil.ReadDir(filepath.Join(util.RepoPath, "data", d.Name()))
		if err != nil {
			log.Fatal(err)
		}
		totalPacks += len(packs)
	}

	fmt.Printf("%d pack files found\n", totalPacks)
	seconds := 0
	fmt.Println("Indexing pack files...")
	for {
		time.Sleep(1 * time.Second)
		seconds += 1
		rate := float64(indexedPacks / seconds)
		remaining := (float64(totalPacks) / rate) / 3600
		fmt.Printf("\r\033[K")
		fmt.Printf("%d packs indexed: %.1f packs/s, %.1f hours remaining, %d blobs indexed", indexedPacks, rate, remaining, indexedBlobs)
	}
}
