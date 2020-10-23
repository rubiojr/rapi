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

### info

Pretty prints basic repository information.

![](images/repository-info.png)

### id

prints restic's repository ID.

    rapi repository id

