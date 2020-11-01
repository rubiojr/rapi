package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/rubiojr/rapi"
	"github.com/rubiojr/rapi/backend"
	"github.com/rubiojr/rapi/internal/errors"
	"github.com/rubiojr/rapi/repository"
	"github.com/rubiojr/rapi/restic"
)

func init() {
	cmd := &cli.Command{
		Name:  "cat",
		Usage: "Print internal objects to stdout",
		Subcommands: []*cli.Command{
			&cli.Command{
				Name:   "blob",
				Action: runCatBlob,
				Flags:  []cli.Flag{},
				Before: func(c *cli.Context) error {
					return setupApp(c)
				},
			}, &cli.Command{
				Name:   "config",
				Action: runCatConfig,
				Flags:  []cli.Flag{},
				Before: func(c *cli.Context) error {
					return setupApp(c)
				},
			}, &cli.Command{
				Name:   "index",
				Action: runCatIndex,
				Flags:  []cli.Flag{},
				Before: func(c *cli.Context) error {
					return setupApp(c)
				},
			}, &cli.Command{
				Name:   "snapshot",
				Action: runCatSnapshot,
				Flags:  []cli.Flag{},
				Before: func(c *cli.Context) error {
					return setupApp(c)
				},
			}, &cli.Command{
				Name:   "key",
				Action: runCatKey,
				Flags:  []cli.Flag{},
				Before: func(c *cli.Context) error {
					return setupApp(c)
				},
			}, &cli.Command{
				Name:   "masterkey",
				Action: runCatMasterKey,
				Flags:  []cli.Flag{},
				Before: func(c *cli.Context) error {
					return setupApp(c)
				},
			},
		},
	}
	appCommands = append(appCommands, cmd)
}

func runCatMasterKey(c *cli.Context) error {
	return runCatFor(c, "masterkey")
}

func runCatKey(c *cli.Context) error {
	if c.Args().Get(0) == "" {
		return errors.Fatal("ID not specified")
	}
	return runCatFor(c, "key")
}

func runCatSnapshot(c *cli.Context) error {
	if c.Args().Get(0) == "" {
		return errors.Fatal("ID not specified")
	}
	return runCatFor(c, "snapshot")
}

func runCatBlob(c *cli.Context) error {
	if c.Args().Get(0) == "" {
		return errors.Fatal("ID not specified")
	}
	return runCatFor(c, "blob")
}

func runCatIndex(c *cli.Context) error {
	if c.Args().Get(0) == "" {
		return errors.Fatal("ID not specified")
	}
	return runCatFor(c, "index")
}

func runCatConfig(c *cli.Context) error {
	return runCatFor(c, "config")
}

func runCatFor(c *cli.Context, tpe string) error {
	ctx := context.Background()
	var err error
	arg0 := c.Args().Get(0)

	var id restic.ID
	if tpe != "masterkey" && tpe != "config" {
		id, err = restic.ParseID(arg0)
		if err != nil {
			if tpe != "snapshot" {
				return errors.Fatalf("unable to parse ID: %v\n", err)
			}

			// find snapshot id with prefix
			id, err = restic.FindSnapshot(ctx, rapiRepo, arg0)
			if err != nil {
				return errors.Fatalf("could not find snapshot: %v\n", err)
			}
		}
	}

	// handle all types that don't need an index
	switch tpe {
	case "config":
		buf, err := json.MarshalIndent(rapiRepo.Config(), "", "  ")
		if err != nil {
			return err
		}

		rapi.Println(string(buf))
		return nil
	case "index":
		buf, err := rapiRepo.LoadAndDecrypt(ctx, nil, restic.IndexFile, id)
		if err != nil {
			return err
		}

		rapi.Println(string(buf))
		return nil
	case "snapshot":
		sn := &restic.Snapshot{}
		err = rapiRepo.LoadJSONUnpacked(ctx, restic.SnapshotFile, id, sn)
		if err != nil {
			return err
		}

		buf, err := json.MarshalIndent(&sn, "", "  ")
		if err != nil {
			return err
		}

		rapi.Println(string(buf))
		return nil
	case "key":
		h := restic.Handle{Type: restic.KeyFile, Name: id.String()}
		buf, err := backend.LoadAll(ctx, nil, rapiRepo.Backend(), h)
		if err != nil {
			return err
		}

		key := &repository.Key{}
		err = json.Unmarshal(buf, key)
		if err != nil {
			return err
		}

		buf, err = json.MarshalIndent(&key, "", "  ")
		if err != nil {
			return err
		}

		rapi.Println(string(buf))
		return nil
	case "masterkey":
		buf, err := json.MarshalIndent(rapiRepo.Key(), "", "  ")
		if err != nil {
			return err
		}

		rapi.Println(string(buf))
		return nil
	case "lock":
		lock, err := restic.LoadLock(ctx, rapiRepo, id)
		if err != nil {
			return err
		}

		buf, err := json.MarshalIndent(&lock, "", "  ")
		if err != nil {
			return err
		}

		rapi.Println(string(buf))
		return nil
	}

	// load index, handle all the other types
	err = rapiRepo.LoadIndex(ctx)
	if err != nil {
		return err
	}

	switch tpe {
	case "pack":
		h := restic.Handle{Type: restic.PackFile, Name: id.String()}
		buf, err := backend.LoadAll(ctx, nil, rapiRepo.Backend(), h)
		if err != nil {
			return err
		}

		hash := restic.Hash(buf)
		if !hash.Equal(id) {
			rapi.Warnf("Warning: hash of data does not match ID, want\n  %v\ngot:\n  %v\n", id.String(), hash.String())
		}

		_, err = os.Stdout.Write(buf)
		return err

	case "blob":
		for _, t := range []restic.BlobType{restic.DataBlob, restic.TreeBlob} {
			if !rapiRepo.Index().Has(id, t) {
				continue
			}

			buf, err := rapiRepo.LoadBlob(ctx, t, id, nil)
			if err != nil {
				return err
			}

			_, err = os.Stdout.Write(buf)
			return err
		}

		return errors.Fatal("blob not found")

	default:
		return errors.Fatal("invalid type")
	}
}
