package restic_test

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/rubiojr/rapi/internal/errors"
	"github.com/rubiojr/rapi/repository"
	"github.com/rubiojr/rapi/restic"
)

func loadIDSet(t testing.TB, filename string) restic.BlobSet {
	f, err := os.Open(filename)
	if err != nil {
		t.Logf("unable to open golden file %v: %v", filename, err)
		return restic.NewBlobSet()
	}

	sc := bufio.NewScanner(f)

	blobs := restic.NewBlobSet()
	for sc.Scan() {
		var h restic.BlobHandle
		err := json.Unmarshal([]byte(sc.Text()), &h)
		if err != nil {
			t.Errorf("file %v contained invalid blob: %#v", filename, err)
			continue
		}

		blobs.Insert(h)
	}

	if err = f.Close(); err != nil {
		t.Errorf("closing file %v failed with error %v", filename, err)
	}

	return blobs
}

func saveIDSet(t testing.TB, filename string, s restic.BlobSet) {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatalf("unable to update golden file %v: %v", filename, err)
		return
	}

	var hs restic.BlobHandles
	for h := range s {
		hs = append(hs, h)
	}

	sort.Sort(hs)

	enc := json.NewEncoder(f)
	for _, h := range hs {
		err = enc.Encode(h)
		if err != nil {
			t.Fatalf("Encode() returned error: %v", err)
		}
	}

	if err = f.Close(); err != nil {
		t.Fatalf("close file %v returned error: %v", filename, err)
	}
}

var updateGoldenFiles = flag.Bool("update", false, "update golden files in testdata/")

const (
	findTestSnapshots = 3
	findTestDepth     = 2
)

var findTestTime = time.Unix(1469960361, 23)

func TestFindUsedBlobs(t *testing.T) {
	repo, cleanup := repository.TestRepository(t)
	defer cleanup()

	var snapshots []*restic.Snapshot
	for i := 0; i < findTestSnapshots; i++ {
		sn := restic.TestCreateSnapshot(t, repo, findTestTime.Add(time.Duration(i)*time.Second), findTestDepth, 0)
		t.Logf("snapshot %v saved, tree %v", sn.ID().Str(), sn.Tree.Str())
		snapshots = append(snapshots, sn)
	}

	for i, sn := range snapshots {
		usedBlobs := restic.NewBlobSet()
		err := restic.FindUsedBlobs(context.TODO(), repo, *sn.Tree, usedBlobs)
		if err != nil {
			t.Errorf("FindUsedBlobs returned error: %v", err)
			continue
		}

		if len(usedBlobs) == 0 {
			t.Errorf("FindUsedBlobs returned an empty set")
			continue
		}

		goldenFilename := filepath.Join("testdata", fmt.Sprintf("used_blobs_snapshot%d", i))
		want := loadIDSet(t, goldenFilename)

		if !want.Equals(usedBlobs) {
			t.Errorf("snapshot %d: wrong list of blobs returned:\n  missing blobs: %v\n  extra blobs: %v",
				i, want.Sub(usedBlobs), usedBlobs.Sub(want))
		}

		if *updateGoldenFiles {
			saveIDSet(t, goldenFilename, usedBlobs)
		}
	}
}

type ForbiddenRepo struct{}

func (r ForbiddenRepo) LoadTree(ctx context.Context, id restic.ID) (*restic.Tree, error) {
	return nil, errors.New("should not be called")
}

func TestFindUsedBlobsSkipsSeenBlobs(t *testing.T) {
	repo, cleanup := repository.TestRepository(t)
	defer cleanup()

	snapshot := restic.TestCreateSnapshot(t, repo, findTestTime, findTestDepth, 0)
	t.Logf("snapshot %v saved, tree %v", snapshot.ID().Str(), snapshot.Tree.Str())

	usedBlobs := restic.NewBlobSet()
	err := restic.FindUsedBlobs(context.TODO(), repo, *snapshot.Tree, usedBlobs)
	if err != nil {
		t.Fatalf("FindUsedBlobs returned error: %v", err)
	}

	err = restic.FindUsedBlobs(context.TODO(), ForbiddenRepo{}, *snapshot.Tree, usedBlobs)
	if err != nil {
		t.Fatalf("FindUsedBlobs returned error: %v", err)
	}
}

func BenchmarkFindUsedBlobs(b *testing.B) {
	repo, cleanup := repository.TestRepository(b)
	defer cleanup()

	sn := restic.TestCreateSnapshot(b, repo, findTestTime, findTestDepth, 0)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		blobs := restic.NewBlobSet()
		err := restic.FindUsedBlobs(context.TODO(), repo, *sn.Tree, blobs)
		if err != nil {
			b.Error(err)
		}

		b.Logf("found %v blobs", len(blobs))
	}
}
