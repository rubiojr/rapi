## Restic IDs

Or [storage IDs](https://restic.readthedocs.io/en/latest/100_references.html#terminology), are what restic uses to name files in a repository. The [official documentation](https://restic.readthedocs.io/en/latest/100_references.html#repository-format) talks about them.

From a practical point of view, it means that you can `sha256sum` a file like `/tmp/restic/data/18/183f81b766a871529418937195e967f0d1b8c5b7d81b24cee601e3f4ec21a388` (a pack file) and the SHA256 hash of the contents will be equal to the file name:

```
sha256sum /tmp/restic/data/18/183f81b766a871529418937195e967f0d1b8c5b7d81b24cee601e3f4ec21a388
183f81b766a871529418937195e967f0d1b8c5b7d81b24cee601e3f4ec21a388  /tmp/restic/data/18/183f81b766a871529418937195e967f0d1b8c5b7d81b24cee601e3f4ec21a388
```

This can be easily used to detect file tampering or corruption in a repository, ensuring the integrity of the blobs contained in a pack or other repository files.
