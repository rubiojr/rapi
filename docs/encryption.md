# File Encryption

The [design document](https://restic.readthedocs.io/en/latest/100_references.html#keys-encryption-and-mac) covers the cryptography details. [Filippo Valsorda's](https://blog.filippo.io/restic-cryptography/) article is also interesting, and a testament to restic's good design.

In its simplest form, our sample repository will have a single encryption key -available in the `keys` directory- so, from a practical point of view, all the interesting files (except they key file which is a plain json file with the encrypted master key) are encrypted with that key.

This is the key that was created after [initializing the repository](/docs/prerequisites.md#create-a-test-repository):

```
cat /tmp/restic/keys/e2e644764f1a5b13eea6c8e351cadd718b7d14c273fe1c5e12a7023ba20c88e4 | jq
```
```json
{
  "created": "2020-09-30T17:43:20.86292862+02:00",
  "username": "rubiojr",
  "hostname": "x390",
  "kdf": "scrypt",
  "N": 32768,
  "r": 8,
  "p": 6,
  "salt": "zRz3EKT9XzWbiU/cVdcYZwSIknD1DcbQylQuS9vKVJkwxu1aldX9bYon8edHQdCM10r+hrT1hPW0GxyyyR62zw==",
  "data": "nzTzHz/T5D4isFeQeMiGBpWScy23vdf6CT9eTltG276KeTTJZ3p2V8WK8WUyEPr0vTKWkOKWkh/X+JjLYVV2242Jo7ZZ/X5T3PVJ+5BdLGXrLrX21lEkq766fVVZg4IgeJt7jl3EL7fOvRZ+ef3w205JdOigWJjcoGDmtj/i2818ZZCWYOP4PHLUPlaLoDGoNgBvfNPwoU4InjSaisy3hQ=="
}
```
_The contents of the key that was created with `restic init`._

**Recommended read: [Is storing key in the backup location really safe?](https://forum.restic.net/t/is-storing-key-in-the-backup-location-really-safe/2021/2)**

The file name, like evey other file in a restic repository, is the file content SHA256 hash.

```
sha256sum /tmp/restic/keys/e2e644764f1a5b13eea6c8e351cadd718b7d14c273fe1c5e12a7023ba20c88e4

e2e644764f1a5b13eea6c8e351cadd718b7d14c273fe1c5e12a7023ba20c88e4 /tmp/restic/keys/e2e644764f1a5b13eea6c8e351cadd718b7d14c273fe1c5e12a7023ba20c88e4
```

## Decrypting repository files

The following example, decrypts the repository configuration (`/tmp/restic/config` in our test repository) -encrypted like (almost) every other file in a restic repository- and writes it to the standard output.

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rubiojr/rapi/examples/util"
)

func main() {
	k := util.FindAndOpenKey()

	// open repository config file
	h, err := os.Open(filepath.Join(util.RepoPath, "config"))
	util.CheckErr(err)

	// decrypt the repo configuration, print it to stdout
	text, err := k.Decrypt(h)
	util.CheckErr(err)
	fmt.Println(string(text))
}
```

If we run the example (make sure you read [prerequisites](/docs/prerequisites.md) first), it'll decrypt `/tmp/restic/config` and print its content:

```
{
  "version": 1,
  "id": "49ccedbed680cc3bebff15d77ee6343dd8b39563869de93abb3f2d0eae3e38d6",
  "chunker_polynomial": "240d51ccba496d"
}
```

## Encrypting files

Once we understand how to use restic's repository keys, we can use restic's encryption to encrypt ordinary files.

The following code encrypts the file available in `examples/data/hello` and saves it to `/tmp/safehello`:

```go
package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/rubiojr/rapi/examples/util"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage: encrypt_file <read_from> <write_to>\n")
		os.Exit(1)
	}
	in, out := os.Args[1], os.Args[2]

	// our sample repo in /tmp/restic will only have one key, let's
	// find it and use it
	k := util.FindAndOpenKey()

	h, err := os.Open(in)
	defer h.Close()
	util.CheckErr(err)

	// Read the file contents
	plain, err := ioutil.ReadAll(h)
	util.CheckErr(err)

	// Encrypt the file using restic's repository master key
	ciphertext := k.Encrypt(plain)

	// Write the resulting ciphertext to the target file
	outf, err := os.Create(out)
	util.CheckErr(err)
	defer outf.Close()
	outf.Write(ciphertext)
}
```

Run it like this (don't forget to read [prerequisites](/docs/prerequisites.md) first):

```
go run examples/encrypt_file.go data/hello /tmp/hello.encrypted
```

If we inspect the new file we'll get the encrypted content, encrypted like every other file backed up by restic:

```
cat /tmp/hello.encrypted
:Vbsy.iw,f?hDx?يX>htwjk
```

If we want to decrypt it, we can use a [slightly different version](/examples/decrypt_file.go) of the decryption example above.

```
go run decrypt_file.go /tmp/hello.encrypted
hello rapi!
```
