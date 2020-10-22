package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/rubiojr/rapi/crypto"
	"github.com/rubiojr/rapi/pack"
	"github.com/rubiojr/rapi/restic"

	"github.com/rubiojr/rapi/examples/util"
)

var blobIndex = map[restic.ID]string{}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run blobs.go <blob-id>")
		os.Exit(1)
	}
	treeBlob := os.Args[1]
	treeBlobID, err := restic.ParseID(treeBlob)

	k := util.FindAndOpenKey()
	// Use a map to index all the blobs so we can easily find
	// which pack contains them later
	indexBlobs(util.RepoPath, k)

	// Find the pack that contains our mp3 tree blob that describes the
	// mp3 file attributes and the content blobs
	mp3PackID, err := restic.ParseID(blobIndex[treeBlobID])
	util.CheckErr(err)
	mp3Tree := loadTreeBlob(util.RepoPath, mp3PackID, treeBlobID, k)
	fmt.Printf("MP3 tree blob for %s found and loaded\n", mp3Tree.Nodes[0].Name)

	// The restored mp3 file will be saved here
	restoredF := "/tmp/restored-mp3.mp3"
	restoredMP3, err := os.Create(restoredF)
	util.CheckErr(err)
	defer restoredMP3.Close()

	// Find all the data blobs that from the mp3 file, decrypt them and write
	// them to the destination file
	for _, cBlob := range mp3Tree.Nodes[0].Content {
		p := blobIndex[cBlob]
		packID, err := restic.ParseID(p)
		util.CheckErr(err)
		found := fetchBlob(util.RepoPath, packID, cBlob, k)
		fmt.Printf("Data blob %s found, decrypting and writting it to %s\n", found.ID.Str(), restoredF)
		content := blobContent(util.RepoPath, found, k)
		restoredMP3.Write(content)
	}

	// Make sure the restored MP3 file SHA256 matches the original's
	// sha256sum examples/data/examples/data/Monplaisir_-_04_-_Stage_1_Level_24.mp3
	buf, err := ioutil.ReadFile(restoredF)
	util.CheckErr(err)
	sum := sha256.Sum256(buf)

	ssum := fmt.Sprintf("%x", sum)
	fmt.Printf("Restored MP3 SHA256: %s\n", ssum)
	fmt.Printf("Orinal MP3 SHA256:   %s\n", util.MP3SHA256)
	if ssum == util.MP3SHA256 {
		fmt.Println("File was restored successfully!")
	} else {
		fmt.Println("Restored MP3 file is invalid")
	}
}

// walk restic's repository data dir and index all the pack files found
func indexBlobs(repoPath string, k *crypto.Key) {
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
func indexBlobsInPack(packID, path string, info os.FileInfo, k *crypto.Key) {
	handle, err := os.Open(path)
	util.CheckErr(err)
	defer handle.Close()
	blobs, err := pack.List(k, handle, info.Size())

	for _, blob := range blobs {
		blobIndex[blob.ID] = packID
	}
}

// decrypts a tree blob and creates a Tree struct instance
func loadTreeBlob(repoPath string, packID, treeBlobID restic.ID, k *crypto.Key) *restic.Tree {
	found := fetchBlob(repoPath, packID, treeBlobID, k)
	bc := blobContent(repoPath, found, k)
	tree := &restic.Tree{}
	err := json.Unmarshal(bc, tree)
	util.CheckErr(err)

	return tree
}

// returns an encrypted restic.Blob instance from a pack
func fetchBlob(repoPath string, packID, blobID restic.ID, k *crypto.Key) *restic.PackedBlob {
	fullPath := filepath.Join(repoPath, "data", packID.DirectoryPrefix(), packID.String())
	handle, err := os.Open(fullPath)
	util.CheckErr(err)
	defer handle.Close()

	info, err := os.Stat(fullPath)
	util.CheckErr(err)

	blobs, err := pack.List(k, handle, info.Size())
	util.CheckErr(err)

	for _, blob := range blobs {
		if blob.ID.Equal(blobID) {
			pb := restic.PackedBlob{Blob: blob, PackID: packID}
			return &pb
		}
	}

	return nil
}

// Decrypts the blob content
func blobContent(repoPath string, blob *restic.PackedBlob, k *crypto.Key) []byte {
	packPath := filepath.Join(repoPath, "data", blob.PackID.DirectoryPrefix(), blob.PackID.String())
	v, err := blob.DecryptFromPack(packPath, k)
	util.CheckErr(err)

	return v
}
