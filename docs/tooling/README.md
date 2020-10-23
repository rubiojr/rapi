# Restic extras

Extra tools to manage Restic repositories, mostly intended for restic developers and advanced users.

**⚠️ Waring**: these tools are in a experimental state now. Do not use in production repositories.

## Installing the tools

```
go get -u github.com/rubiojr/cmd/rapi/...
```

## Available tools

## repository

### info

Pretty prints basic repository information.

![](images/repository-info.png)

### id

prints restic's repository ID.

    rapi repository id

