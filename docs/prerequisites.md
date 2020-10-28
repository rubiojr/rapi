# Prerequisites

## Install Restic and other tools used

You'll want the Restic binary to be available to play around and do some of the things the code in this guide doesn't cover.

I've been using Restic 0.10 while writing this and it's the latest and recommended version, but the previous `0.9.6` (that has been available for a while) should also work.

[jq](https://stedolan.github.io/jq) (`apt-get install jq`) is also used to prettify JSON output and some Bash one-liners. Every other command line tool used should be available in any modern Linux distribution or macOS with the developer tools installed.

## Create a test repository

I've added a sample credentials file to the `examples` directory in this reopsitory. Use it, it'll make it easier to follow this guide if you use the password and repository path provided.

The `creds` file is a script that exports two environment variables:

```
SCRIPT=$(readlink -f "$0")
BASE_PATH="$(dirname "$SCRIPT")/.."
export RESTIC_REPOSITORY=$BASE_PATH/tmp/restic
export RESTIC_PASSWORD=test
```

To create the repository:

```
./script/init-test-repo
```

That'll create a `tmp/restic` repository using the password `test`.
**Don't use that repository to backup valuable data**: the password is not recommended for production use and we will intentionally damage the Restic repository contents while following this guide.

I've also included some sample data that you can backup now:

```
restic backup examples/data

repository af668d00 opened successfully, password is correct
created new cache in /home/rubiojr/.cache/restic

Files:           3 new,     0 changed,     0 unmodified
Dirs:            4 new,     0 changed,     0 unmodified
Added to the repo: 12.286 MiB

processed 3 files, 12.283 MiB in 0:00
snapshot 7eeaf82d saved
```

## Install Go

I've been using Go >= 1.15 to compile and test the examples.
