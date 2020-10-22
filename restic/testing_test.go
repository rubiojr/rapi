package restic_test

import (
	"context"
	"testing"
	"time"

	"github.com/rubiojr/rapi/internal/checker"
	"github.com/rubiojr/rapi/repository"
	"github.com/rubiojr/rapi/restic"
)

var testSnapshotTime = time.Unix(1460289341, 207401672)

const (
	testCreateSnapshots = 3
	testDepth           = 2
)

func TestCreateSnapshot(t *testing.T) {
	repo, cleanup := repository.TestRepository(t)
	defer cleanup()

	for i := 0; i < testCreateSnapshots; i++ {
		restic.TestCreateSnapshot(t, repo, testSnapshotTime.Add(time.Duration(i)*time.Second), testDepth, 0)
	}

	snapshots, err := restic.LoadAllSnapshots(context.TODO(), repo, restic.NewIDSet())
	if err != nil {
		t.Fatal(err)
	}

	if len(snapshots) != testCreateSnapshots {
		t.Fatalf("got %d snapshots, expected %d", len(snapshots), 1)
	}

	sn := snapshots[0]
	if sn.Time.Before(testSnapshotTime) || sn.Time.After(testSnapshotTime.Add(testCreateSnapshots*time.Second)) {
		t.Fatalf("timestamp %v is outside of the allowed time range", sn.Time)
	}

	if sn.Tree == nil {
		t.Fatalf("tree id is nil")
	}

	if sn.Tree.IsNull() {
		t.Fatalf("snapshot has zero tree ID")
	}

	checker.TestCheckRepo(t, repo)
}

func BenchmarkTestCreateSnapshot(t *testing.B) {
	repo, cleanup := repository.TestRepository(t)
	defer cleanup()

	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		restic.TestCreateSnapshot(t, repo, testSnapshotTime.Add(time.Duration(i)*time.Second), testDepth, 0)
	}
}
