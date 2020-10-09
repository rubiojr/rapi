# Pack files

Pack files -files in the `data` directory inside the restic repository- contain one or more blobs of data. Everything under `data` is a pack file, which means that when backing up your files, they will be split -if bigger than 512 KiB- and added to one of these pack files. Pack files group data and tree blobs [to avoid the overhead of seeking too many small files](https://github.com/restic/restic/issues/347#issuecomment-154853061).
It also makes the process of locating the encrypted blob (or blobs) that form your backed up files a bit trickier ;).

The [pack format documentation](https://restic.readthedocs.io/en/latest/100_references.html#pack-format) documentation explains pack files in greater detail.

Let's see how this works in practice.

## Backing up a large mp3

Let's create a test repository and backup the mp3 file available in `examples/data/Monplaisir_-_04_-_Stage_1_Level_24.mp3` (retrieved from the [free music archive](https://freemusicarchive.org/music/Monplaisir/Heat_of_the_Summer/Monplaisir_-_Monplaisir_-_Heat_of_the_Summer_-_04_Stage_1_Level_24)).

```
# remove the old test repo if present
rm -rf /tmp/restic
source examples/creds
restic init
restic backup examples/data/Monplaisir_-_04_-_Stage_1_Level_24.mp3
```

This is the output from the backup command:

```
repository 8bbdd885 opened successfully, password is correct
created new cache in /home/rubiojr/.cache/restic

Files:           1 new,     0 changed,     0 unmodified
Dirs:            2 new,     0 changed,     0 unmodified
Added to the repo: 12.284 MiB

processed 1 files, 12.283 MiB in 0:00
snapshot b518efc8 saved
```

A few interesting things happened here:

* A new file was backed up, the mp3.
* Two new directories have been added, `examples`, and `examples/data`. Restic adds a blob to the pack files for every new or changed directory, we'll see that in action later.

If we list the repository files under `data` now, this is what we get:

```
find /tmp/restic/data -type f|xargs ls -1 -sh
1.7M /tmp/restic/data/22/2234015a3c05e32c1d101b9a0674d9a7eb3213bd4fa2e0881ec8dcf5edfc2778
3.3M /tmp/restic/data/22/228e40e21ea582ed22a7f5ae1ce638860bfdf2d61fe7f078433534d38d896973
4.0K /tmp/restic/data/8d/8d5d29a8f99ea67d5f5ac2c2129b9c94e8804682eaee0cf670a5e37aa8f26580
7.4M /tmp/restic/data/d1/d18b0ca52803e8885c3931309d15ef4b05b62fcba1a31efa230ea10eecdd11c2
```

_Note that the file names (storage IDs) of your repository (and maybe the number of pack files) **will be different for you**, as your repository will have a different configuration, and that means file chunking and deduplication will work differently, among other things. More on this later in [file chunking](/docs/chunking.md)._

Interesting enough, we have three large pack files (several MiB) and one small.

restic [splits files bigger than 512KiB](https://restic.readthedocs.io/en/latest/100_references.html#backups-and-deduplication), so it seems the mp3 file was split and the blobs where spread among three different pack files. What about the smaller pack file? Let's try to figure it out with some code.

The [pack.go](/examples/pack.go) example walks the `data` directory and for every file found inspects the pack file and pretty prints the structure:

```
go run examples/pack.go
Pack file:  /tmp/restic/data/22/2234015a3c05e32c1d101b9a0674d9a7eb3213bd4fa2e0881ec8dcf5edfc2778
  Size:  1.8 MB
  Blobs:  1
    data: db646f6b5566801180cb310f6abcc4b417cc9d51a449748849e44f084350968e (1.8 MB)
Pack file:  /tmp/restic/data/22/228e40e21ea582ed22a7f5ae1ce638860bfdf2d61fe7f078433534d38d896973
  Size:  3.4 MB
  Blobs:  2
    data: 64c6b4964a0b01b6e11f1129e2071fe0093480b636fe9b63138a1fb1c5c613d4 (1.4 MB)
    data: 68692441140ede9315df14ed9973c096288766e548a9b6c03acd8a9d32991d6e (2.0 MB)
Pack file:  /tmp/restic/data/8d/8d5d29a8f99ea67d5f5ac2c2129b9c94e8804682eaee0cf670a5e37aa8f26580
  Size:  1.8 kB
  Blobs:  3
    tree: 0480151d0705d3a9741ee904d5b2219ef465b03ccd33bf77097e28eec9ae1b69 (856 B)
    tree: e80274e4b1c276203f3b6cffff162cb8babd994eaec45b3c128a1768c20e7ee2 (413 B)
    tree: 9924436182bf9d5b8454f0acd41f11160d254cde554c52a6cccbe2b57d422856 (417 B)
Pack file:  /tmp/restic/data/d1/d18b0ca52803e8885c3931309d15ef4b05b62fcba1a31efa230ea10eecdd11c2
  Size:  7.7 MB
  Blobs:  4
    data: 21c11cc8c5fa5607f2311e0d9b5ef6798faf48c6a11772ca430122cae3e13b0a (1.4 MB)
    data: 07f659f23cf20404515e598d2c9f9d4aab0cc909993474561d96e94835abc321 (783 kB)
    data: 81e868bbc0beefc29754be3f5495c4ba2e194f9ab7c203a3a7a3ac6ca2101510 (1.2 MB)
    data: af3e4cd790c5d77026bacfb0abecd6306f1fd978c97a877e13521a8e5a4c3ded (4.3 MB)
```

_Again, the output for you will be different._

Pack file `8d5d29` contains three trees, and every other pack file contains data blobs only, probably the binary chunks of the mp3 file. Let's see.

We know from the official documentation that [trees are JSON blobs](https://restic.readthedocs.io/en/latest/100_references.html#trees-and-data), so we can easily print them using `restic cat blob`. Let's print one of the tree blobs from the previous output:

```
restic cat blob 0480151d0705d3a9741ee904d5b2219ef465b03ccd33bf77097e28eec9ae1b69 | jq
```

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

Nice! that tree blob represents the mp3 file we backed up and and ordered lists all the binary blobs required to reconstruct the file. We can use a bash one-liner to do it:

```
restic cat blob 0480151d0705d3a9741ee904d5b2219ef465b03ccd33bf77097e28eec9ae1b69 | \
       jq -r '.nodes | .[0].content[]' | \
       xargs -I{} restic cat blob {} >> /tmp/restored.mp3
```

```
file /tmp/restored.mp3
/tmp/restored.mp3: Audio file with ID3 version 2.4.0, contains:MPEG ADTS, layer III, v1, 320 kbps, 44.1 kHz, JntStereo
```

Sweet, play that file, it should be the mp3 file you backed up.
The SHA256 hash of the recovered file should also match the original's, let's double check that:

```
sha256sum /tmp/recovered.mp3 examples/data/Monplaisir_-_04_-_Stage_1_Level_24.mp3

01d4bac715e7cc70193fdf70db3c5022d0dd5f33dacd6d4a07a2747258416338  /tmp/recovered.mp3
01d4bac715e7cc70193fdf70db3c5022d0dd5f33dacd6d4a07a2747258416338  examples/data/Monplaisir_-_04_-_Stage_1_Level_24.mp3
```

Boom! verified.

We previously mentioned that the backup output said it had found two new directories. These are represented as trees and added as tree blobs to the pack files in the repository. The output from the code example above listed two more tree blobs, in addition to the mp3 file tree blob. Let's print them:

```
restic cat blob e80274e4b1c276203f3b6cffff162cb8babd994eaec45b3c128a1768c20e7ee2 | jq
```

```json
{
  "nodes": [
    {
      "name": "data",
      "type": "dir",
      "mode": 2147484157,
      "mtime": "2020-10-01T18:55:15.518116994+02:00",
      "atime": "2020-10-01T18:55:15.518116994+02:00",
      "ctime": "2020-10-01T18:55:15.518116994+02:00",
      "uid": 1000,
      "gid": 1000,
      "user": "rubiojr",
      "group": "rubiojr",
      "inode": 14562296,
      "device_id": 64769,
      "content": null,
      "subtree": "0480151d0705d3a9741ee904d5b2219ef465b03ccd33bf77097e28eec9ae1b69"
    }
  ]
}
```

Which is the `data` directory tree blob that has one subtree, pointing to the mp3 tree blob.

```
restic cat blob 9924436182bf9d5b8454f0acd41f11160d254cde554c52a6cccbe2b57d422856
```

```json
{
  "nodes": [
    {
      "name": "examples",
      "type": "dir",
      "mode": 2147484157,
      "mtime": "2020-10-05T17:21:57.947433104+02:00",
      "atime": "2020-10-05T17:21:57.947433104+02:00",
      "ctime": "2020-10-05T17:21:57.947433104+02:00",
      "uid": 1000,
      "gid": 1000,
      "user": "rubiojr",
      "group": "rubiojr",
      "inode": 14562268,
      "device_id": 64769,
      "content": null,
      "subtree": "e80274e4b1c276203f3b6cffff162cb8babd994eaec45b3c128a1768c20e7ee2"
    }
  ]
}
```

The `examples` directory tree blob also with one `subtree`, that points to the `data` directory tree blob.

As you can see, with a bit more effort, we could adapt [pack.go](/examples/pack.go) to iterate over all the pack files in a repository, find all the tree blobs that contain mp3 files, restore them, load them into memory and play them, etc.
