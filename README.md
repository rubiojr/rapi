# Restic by Example

[![CI Status](https://github.com/rubiojr/rapi/workflows/ci/badge.svg)](https://github.com/rubiojr/rapi/actions)
[![Go ReportCard](http://goreportcard.com/badge/rubiojr/rapi)](http://goreportcard.com/report/rubiojr/rapi)

I'm trying to learn how [restic](https://github.com/restic/restic) works, and this repository relates my journey.

## Tooling

This repository also provides some useful tooling for restic repositories. Please see the [tooling documentation](/docs/tooling/README.md).

## Target audience

This is not a user's guide, [the official documentation](https://restic.readthedocs.io/en/latest/010_introduction.html) is quite good at it.

This guide is for those of you willing to contribute to restic eventually, build tooling to augment restic’s ecosystem, or self-serve if something terrible happens with your data, empowering you to create the necessary tooling to troubleshot/fix/workaround things.

## The guide

The guide only covers core restic operations. Things like the virtual filesystem, remote repository backends, the cache and some other things were intentionally left out. I may include then in a future advanced restic internal's guide.

[0. Intro](/docs/intro.md)

[1. Prerequisites](/docs/prerequisites.md)

[2. Restic IDs](/docs/ids.md)

[3. File Encryption](/docs/encryption.md)

[4. Pack Files](/docs/packfiles.md)

[5. Blobs](/docs/blobs.md)

[6. The Index](/docs/index.md) STARTED

[7. File Chunking](/docs/chunking.md) TODO

[8. Deduplication](/docs/deduplication.md) TODO

[9. Recommended Reading](/docs/reading.md)

**Note:** this **unofficial** guide is a **work in progress**, from a random guy who's never contributed to restic before. You should take all this with a grain of salt until it's been reviewed by a few more eyes and this note is removed.

I may have introduced new bugs in restic's code while adapting it for this guide. Code or guide reviews and fixes for any errors found are greatly appreciated.

##
## Credits

Most of the API code in this repository comes from [restic](https://github.com/restic/restic). I've modified it so it can be used externally (moving code from `internal` to external packages) and added some helper functions that should make it a bit easier to use when dealing with local restic repositories.

Not all the code from restic's repository has been imported, just enough code to be able to read, write, decrypt and encrypt local repository files.


