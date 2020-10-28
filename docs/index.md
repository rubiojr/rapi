# The Index

Restic's [index files](https://restic.readthedocs.io/en/stable/100_references.html#indexing) contain information about data and tree blobs and the packs they are contained in, and store this information in the repository.

The first question we should answer perhaps is: **why does Restic need an index?**

Quoting restic's [souce code comments](https://github.com/restic/restic/pull/3006/files):

> In large repositories, millions of blobs are stored in the repository
> and restic needs to store an index entry for each blob in memory for
> most operations.

With thousands, maybe millions of files to read and analyze, the purpose of an index is to improve the speed of data retrieval operations.

## Why Restic needs an index

Imagine we have a repository with hundreds of thousands of [pack files](/docs/packfiles.md) in the `data` directory, storing millions of blobs, and we want to restore or read a single file that was large enough to be chunked ([a big MP3 for example](/docs/blobs.md)) when it was backed up, resulting in several data blobs stored in different pack files.

Assuming we only know the file's name (`my_awesome.mp3`), to be able to find the required data blobs to restore it, we'd need to:

* Walk the `data` directory in our repository and find the pack file that has the tree blob describing our MP3 file, by:
  * Reading the [pack file header](https://restic.readthedocs.io/en/stable/100_references.html#pack-format) and decrypt it.
  * Reading each tree blob (if there are any) to check if the file name matches the one we want.
* Once we have the tree blob, we have the list of blobs that form our MP3 file. Imagine the MP3 file we're looking for is formed by three different blobs: A, B and C (blob IDs will be SHA256 hashes, not simple letters). We'd need to walk the `data` directory again to find the pack files that contain blobs A, B and C.

As you can imagine, in a remote repository (stored in AWS S3 for example) with 300_000 pack files and millions of blobs, this could take hours or even days, as we'd need to use the network to read every [pack file header](https://restic.readthedocs.io/en/stable/100_references.html#pack-format) that contains the information about the blobs stored and do it twice, unless we keep some sort of data structure in memory (or disk) that we can query to figure out in which pack file blobs A, B and C are stored.

To make the process of finding pack file that contains a given blob much faster, Restic builds an index that is persisted to disk.

Let's create a test repository and backup some files to illustrate this.

```
./scrtip/init-test-repo
source examples/creds
restic backup examples/data/hola
```

If we backup one of the example files included:

```
$ restic backup examples/data/hello
repository 5f8e4f1a opened successfully, password is correct

Files:           1 new,     0 changed,     0 unmodified
Dirs:            2 new,     0 changed,     0 unmodified
Added to the repo: 1.134 KiB

processed 1 files, 12 B in 0:00
snapshot 24fcd64d saved
```

we'll see that Restic has created a new index file:

```
$ ls /tmp/restic/index
849b6e4820ba805593af7005d69d41614c95becd3704eabe8e9fd5e0a7b379ae
```

We can dump that index file to understand what Restic stored in its index:

```
$ restic cat index 849b6e4820ba805593af7005d69d41614c95becd3704eabe8e9fd5e0a7b379ae | jq
```


```json
{
  "packs": [
    {
      "id": "ceb0e35690e71cf44cb496d2f8075dccdad1f51acd1d1641215718f65b6eb464",
      "blobs": [
        {
          "id": "648984103f092cb65e89b021642e494bdd256eb792b0ef932ae35bbbe8f4c874",
          "type": "data",
          "offset": 0,
          "length": 44
        }
      ]
    },
    {
      "id": "8b5eca3e16555ad097c01608ec0f42aa4137b90c582a483219d61bbef32a68b6",
      "blobs": [
        {
          "id": "bc64fad40cea9fc8bbbc54c15d61e0fc2393d15918ef1b3a35644cd1a5e46763",
          "type": "tree",
          "offset": 828,
          "length": 417
        },
        {
          "id": "441cab31833f3e5b828909d786d74b100cc475feb2546a7b2b55804a081b0b28",
          "type": "tree",
          "offset": 0,
          "length": 415
        },
        {
          "id": "ec4b5189e306ede147c17590aa0079976a6cd9c3d29a6fe6e4095833ec95f906",
          "type": "tree",
          "offset": 415,
          "length": 413
        }
      ]
    }
  ]
}
```

We can easily see that when we backed up `examples/data/hello`, Restic created two pack files (`885eca...` and `ceb0e3...`), three tree blobs (two for the directories and one for the `hello` file) and one data blob (the hello file content). We can list the pack files in the repository to double check this:

```
find /tmp/restic/data -type f
/tmp/restic/data/8b/8b5eca3e16555ad097c01608ec0f42aa4137b90c582a483219d61bbef32a68b6
/tmp/restic/data/ce/ceb0e35690e71cf44cb496d2f8075dccdad1f51acd1d1641215718f65b6eb464
```

Those are present in the index. We can also check the contents of the `hello` file for example, also listed in the index:

```
$ restic cat blob 648984103f092cb65e89b021642e494bdd256eb792b0ef932ae35bbbe8f4c874
repository 5f8e4f1a opened successfully, password is correct
hello rapi!
```

A few important things to keep in mind:

* Restic adds packs and blobs to the index when we run `restic backup`. Given that it needs to walk the filesystem to back things up, it's a good moment to index blobs and pack files created so we can query them later, without having to walk the repository `data/` directory again.
* Every time we run `backup`, **at least one** new index file is created. [Index files size is kept below 8MiB](https://restic.readthedocs.io/en/stable/100_references.html#indexing), so restic may create more than one index file if we're backing up a very large number of files (a single index file can contain more than 60_000 blob references).
* Index files are immutable, meaning that once they're written to the repository they'll never be modified, but other Restic commands (like `prune` or `rebuild-index`) may combine/compact/repack them reducing the number of index files required. If we run `restic backup` to backup a small file every day of the year, we'd end up with 365 index files that can easily be repacked into a single file if we run `restic rebuild-index` (bear in mind that `rebuild-index` is very expensive in large repositories).

## Examples

I've added a naive but simple implementation of what an in memory index using a map would look like to [index_blobs.go](/examples/index_blobs.go), without using Restic's data structures:

![](/docs/images/index.gif)

Restic solved this problem with an in-memory index that is persisted (as encrypted JSON files) to the disk, plus the necessary abstractions to save, access and cache the index from a number of different backends (S3, Backblaze, local filesystem, etc).

Here's what accessing the index would look like, using Restic's internal API.

First we load the index into memory:

```go
	repo, err := rapi.OpenRepository(rapi.DefaultOptions)
	util.CheckErr(err)
	repo.LoadIndex(context.Background())
	index := repo.Index()
```

That'll download (if required), cache and read all the index files and return the master index, which is just a collection of all the index files available in the repository.

Once we have it loaded, we can use it to do the things the index was designed for: quickly find available blobs and the pack files where they are contained.

Imagine we know a certain blob ID and we want to figure out the pack file that hosts it:

```go
dataBlob, _ := restic.ParseID("49bec000d8b727d3c50e8687e71b0a8deb65a84933f6c4dbbe07513ed39919cc")
found := index.Has(dataBlob, restic.DataBlob)
fmt.Printf("Is blob 49bec0 available in the index? %t\n", found)
for _, blob := range index.Lookup(dataBlob, restic.DataBlob) {
	fmt.Printf("Blob %s found in pack file %s\n", blob.ID, blob.PackID)
}
```
_The full source code for the example can be found in [restic_index.go](/examples/restic_index.go)._

Armed with that information, we can now build tools that perform better when figuring out where blobs are stored is required, even with very large Restic repositories.

## Related reading

* Indexing strategies: https://github.com/restic/restic/issues/2523
* Recent index optimizations: https://github.com/restic/restic/pull/2781
* Rebuilding index while pruning: https://github.com/restic/restic/pull/2842
* Re-implementing prune: https://github.com/restic/restic/pull/2718
