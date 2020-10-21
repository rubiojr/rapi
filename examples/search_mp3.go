/*
	search_mp3.go
*/
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rubiojr/rapi/blob"
	"github.com/rubiojr/rapi/key"
	"github.com/rubiojr/rapi/pack"
	"github.com/rubiojr/rapi/restic"

	"github.com/rubiojr/rapi/examples/util"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run blobs.go <blob-id>")
		os.Exit(1)
	}
	fileName := os.Args[1]

	k := util.FindAndOpenKey()
	// Use a map to index all the blobs so we can easily find
	// which pack contains them later
	treeBlob := locateTreeBlobFor(fileName, util.RepoPath, k)
	if treeBlob == nil {
		fmt.Printf("File %s not found in the repository!\n", fileName)
		os.Exit(1)
	}
	fmt.Println(treeBlob.Nodes[0])
}

// walk restic's repository data dir and index all the pack files found
func locateTreeBlobFor(fileName, repoPath string, k *key.Key) *restic.Tree {
	dataDir := filepath.Join(repoPath, "data")
	var treeBlob *restic.Tree
	finderFunc := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		treeBlob = analyzePack(fileName, info.Name(), path, info, k)
		if treeBlob != nil {
			// stop walking the tree
			return io.EOF
		}

		return nil
	}
	err := filepath.Walk(dataDir, finderFunc)
	if err != nil && err != io.EOF {
		panic(err)
	}

	return treeBlob
}

// Add all the blob IDs found in a pack to the index map
func analyzePack(wanted, packID, path string, info os.FileInfo, k *key.Key) *restic.Tree {
	handle, err := os.Open(path)
	util.CheckErr(err)
	defer handle.Close()
	blobs, err := pack.List(k.Master, handle, info.Size())
	util.CheckErr(err)

	for _, b := range blobs {
		if b.Type != blob.TreeBlob {
			continue
		}
		pid, err := restic.ParseID(packID)
		util.CheckErr(err)
		treeBlob := loadTreeBlob(path, pid, b.ID, k)
		if treeBlob.Nodes[0].Name == wanted {
			return treeBlob
		}
	}

	return nil
}

// Decrypts the blob content
func blobContent(packPath string, blob *blob.Blob, k *key.Key) []byte {
	v, err := blob.DecryptFromPack(packPath, k.Master)
	util.CheckErr(err)

	return v
}

// decrypts a tree blob and creates a Tree struct instance
func loadTreeBlob(packPath string, packID, treeBlobID restic.ID, k *key.Key) *restic.Tree {
	found := fetchBlob(packPath, packID, treeBlobID, k)
	bc := blobContent(packPath, found, k)
	tree := &restic.Tree{}
	err := json.Unmarshal(bc, tree)
	util.CheckErr(err)

	return tree
}

func fetchBlob(packPath string, packID, blobID restic.ID, k *key.Key) *blob.Blob {
	handle, err := os.Open(packPath)
	util.CheckErr(err)
	defer handle.Close()

	info, err := os.Stat(packPath)
	util.CheckErr(err)

	blobs, err := pack.List(k.Master, handle, info.Size())
	util.CheckErr(err)

	for _, blob := range blobs {
		if blob.ID.Equal(blobID) {
			blob.PackID = packID
			return &blob
		}
	}

	return nil
}
