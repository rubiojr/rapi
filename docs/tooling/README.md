# Restic extras

Extra tools to manage Restic repositories, mostly intended for restic developers and advanced users.

**⚠️ Waring**: these tools are in a experimental state now. Do not use in production repositories.

## Installing the tools

### From source

```
GO111MODULE=on go get github.com/rubiojr/rapi/cmd/rapi@latest
```

### Binaries

No binaries available for the moment.

## Available tools

## repository

    rapi repository info

Prints basic repository information.

![](images/repository-info.png)

    rapi repository id

Prints restic's repository ID.

![](images/repository-id.png)

## snapshots

    rapi snapshot info

Prints basic snapshot information retrieved from the latest available snapshot.

![](images/snapshot-info.png)