# Blobs

In the [pack files chapter](/docs/packfiles.md), we learned that pack files will have [one or more tree or data encrypted blobs](https://restic.readthedocs.io/en/latest/100_references.html#pack-format) that will contain information about our filesystem (tree blobs) or raw data (data blobs) from our backed up files.
Each backed up file will result in one or more data blobs added to one of these pack files, that we can read and decrypt to access the original content.

It was demonstrated how we can backup a big MP3 file and use some code to list the blobs in the pack files added to the restic repository, then restore that mp3 using the low level `restic cat` sub-command.

From the previous chapter also, we discovered that blob `0480151d0705d3a9741ee904d5b2219ef465b03ccd33bf77097e28eec9ae1b69` was the tree blob that has the information about our MP3 file.
When the `Monplaisir_-_04_-_Stage_1_Level_24.mp3` was backed up, restic split the file in 7 different [variable length blobs](https://restic.readthedocs.io/en/latest/100_references.html#backups-and-deduplication), encrypted them and added them to different pack files:

```
restic cat blob 0480151d0705d3a9741ee904d5b2219ef465b03ccd33bf77097e28eec9ae1b69 | jq
```

was used to print the MP3 tree blob that contains all the information required to reconstruct the original file from those 7 blobs:

```json
{
  "nodes": [
    {
      "name": "Monplaisir_-_04_-_Stage_1_Level_24.mp3",
      "type": "file",
      "mode": 436,
      "mtime": "2020-10-01T18:54:25.822030931+02:00",
      "atime": "2020-10-01T18:54:25.822030931+02:00",
      "ctime": "2020-10-01T18:54:52.534077041+02:00",
      "uid": 1000,
      "gid": 1000,
      "user": "rubiojr",
      "group": "rubiojr",
      "inode": 13317865,
      "device_id": 64769,
      "size": 12879358,
      "links": 1,
      "content": [
        "21c11cc8c5fa5607f2311e0d9b5ef6798faf48c6a11772ca430122cae3e13b0a",
        "64c6b4964a0b01b6e11f1129e2071fe0093480b636fe9b63138a1fb1c5c613d4",
        "68692441140ede9315df14ed9973c096288766e548a9b6c03acd8a9d32991d6e",
        "07f659f23cf20404515e598d2c9f9d4aab0cc909993474561d96e94835abc321",
        "db646f6b5566801180cb310f6abcc4b417cc9d51a449748849e44f084350968e",
        "81e868bbc0beefc29754be3f5495c4ba2e194f9ab7c203a3a7a3ac6ca2101510",
        "af3e4cd790c5d77026bacfb0abecd6306f1fd978c97a877e13521a8e5a4c3ded"
      ]
    }
  ]
}
```

The `content` array is an ordered list of encrypted blobs that form the `Monplaisir_-_04_-_Stage_1_Level_24.mp3` file, meaning that if we iterate that list sequentially, read each blob from the pack that contains it, decrypt it and write the resulting plaintext to a file in order, we'll get our MP3 back. That's what we did with this shell one-liner:

```
restic cat blob 0480151d0705d3a9741ee904d5b2219ef465b03ccd33bf77097e28eec9ae1b69 | \
       jq -r '.nodes | .[0].content[]' | \
       xargs -I{} restic cat blob {} >> /tmp/restored.mp3
```

We'll now do the same but using [our own code](/examples/blobs.go):

```
go run examples/blobs.go 0480151d0705d3a9741ee904d5b2219ef465b03ccd33bf77097e28eec9ae1b69

MP3 tree blob for Monplaisir_-_04_-_Stage_1_Level_24.mp3 found and loaded
Data blob 21c11cc8 found, decrypting and writting it to /tmp/restored-mp3.mp3
Data blob 64c6b496 found, decrypting and writting it to /tmp/restored-mp3.mp3
Data blob 68692441 found, decrypting and writting it to /tmp/restored-mp3.mp3
Data blob 07f659f2 found, decrypting and writting it to /tmp/restored-mp3.mp3
Data blob db646f6b found, decrypting and writting it to /tmp/restored-mp3.mp3
Data blob 81e868bb found, decrypting and writting it to /tmp/restored-mp3.mp3
Data blob af3e4cd7 found, decrypting and writting it to /tmp/restored-mp3.mp3
Restored MP3 SHA256: 01d4bac715e7cc70193fdf70db3c5022d0dd5f33dacd6d4a07a2747258416338
Orinal MP3 SHA256:   01d4bac715e7cc70193fdf70db3c5022d0dd5f33dacd6d4a07a2747258416338
File was restored successfully!
```

Given that we know from the [pack files chapter](/docs/packfiles.md) that `0480151d0705d3a9741ee904d5b2219ef465b03ccd33bf77097e28eec9ae1b69` is the tree blob ID that contains the MP3 file metadata, that'll be enough information to retrieve and decrypt all the data blobs that form the MP3 file content.

First, we want to index all the blobs in every pack file available in the repository. We need to find 7 different data blobs stored in different pack files, so the index will speed things up. Much more complex indexing is also part of restic's source code, we'll talk about that in [the index chapter](/docs/index.md).

```go
	// Use a map to index all the blobs so we can easily find
	// which pack contains them later
	indexBlobs(util.RepoPath, k)
```

For simplicity, we're using a map to store the blob ID as a key and the pack ID where it's been stored as the value.

`indexBlobs` simply walks the filesystem and for every pack file found, lists the blobs in that pack file and adds them to our index:

```go
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
		blobIndex[blob.ID] = packID
	}
}
```

Once we have our simple index, we retrieve the pack ID of the pack that contains the MP3 tree blob and load it.

```go
	// Find the pack that contains our mp3 tree blob that describes the
	// mp3 file attributes and the content blobs
	mp3PackID, err := restic.ParseID(blobIndex[treeBlobID])
	util.CheckErr(err)
	mp3Tree := loadTreeBlob(util.RepoPath, mp3PackID, treeBlobID, k)
	fmt.Printf("MP3 tree blob for %s found and loaded\n", mp3Tree.Nodes[0].Name)
```

`loadTreeBlob` reads the tree blob from the pack file and decrypts it's content, so we get the JSON metadata that describes the MP3 file (as shown earlier in this chapter) that is unmarshalled to get a restic.Tree instance:

```go
// decrypts a tree blob and creates a Tree struct instance
func loadTreeBlob(repoPath string, packID, treeBlobID restic.ID, k *key.Key) *restic.Tree {
	found := fetchBlob(repoPath, packID, treeBlobID, k)
	bc := blobContent(repoPath, found, k)
	tree := &restic.Tree{}
	err := json.Unmarshal(bc, tree)
  util.CheckErr(err)

	return tree
}
```

`fetchBlob` simply reads the encrypted blob from the pack file, so we can decrypt it later.

```go
// returns an encrypted blob.Blob instance from a pack
func fetchBlob(repoPath string, packID, blobID restic.ID, k *key.Key) *blob.Blob {
	fullPath := filepath.Join(repoPath, "data", packID.DirectoryPrefix(), packID.String())
	handle, err := os.Open(fullPath)
	util.CheckErr(err)
	defer handle.Close()

	info, err := os.Stat(fullPath)
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
```

Once we have the tree blob instance (`mp3Tree`), we have access to the ordered list of data blobs that form the mp3 file, so we can iterate over that list, decrypt every data blob, and write it to the destination file, in order:

```go
	// The restored mp3 file will be saved here
	restoredF := "/tmp/restored-mp3.mp3"
	restoredMP3, err := os.Create(restoredF)
  util.CheckErr(err)
  defer restoredMP3.Close()

	for _, cBlob := range mp3Tree.Nodes[0].Content {
		p := blobIndex[cBlob]
		packID, err := restic.ParseID(p)
		util.CheckErr(err)
		found := fetchBlob(util.RepoPath, packID, cBlob, k)
		fmt.Printf("Data blob %s found, decrypting and writting it to %s\n", found.ID.Str(), restoredF)
		content := blobContent(util.RepoPath, found, k)
		restoredMP3.Write(content)
	}
```

Now that we have the MP3 file restored, we can optionally double check the SHA256 of the new file matches the original MP3 SHA256:

```go
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
```

The full working example can be found in [examples/blobs.go](/examples/blobs.go).
