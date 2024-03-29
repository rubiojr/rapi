package restic

import (
	"context"
	"sync"

	"github.com/rubiojr/rapi/internal/ui/progress"
	"golang.org/x/sync/errgroup"
)

// TreeLoader loads a tree from a repository.
type TreeLoader interface {
	LoadTree(context.Context, ID) (*Tree, error)
	LookupBlobSize(id ID, tpe BlobType) (uint, bool)
}

// FindUsedBlobs traverses the tree ID and adds all seen blobs (trees and data
// blobs) to the set blobs. Already seen tree blobs will not be visited again.
func FindUsedBlobs(ctx context.Context, repo TreeLoader, treeIDs IDs, blobs BlobSet, p *progress.Counter) error {
	var lock sync.Mutex

	wg, ctx := errgroup.WithContext(ctx)
	treeStream := StreamTrees(ctx, wg, repo, treeIDs, func(treeID ID) bool {
		// locking is necessary the goroutine below concurrently adds data blobs
		lock.Lock()
		h := BlobHandle{ID: treeID, Type: TreeBlob}
		blobReferenced := blobs.Has(h)
		// noop if already referenced
		blobs.Insert(h)
		lock.Unlock()
		return blobReferenced
	}, p)

	wg.Go(func() error {
		for tree := range treeStream {
			if tree.Error != nil {
				return tree.Error
			}

			lock.Lock()
			for _, node := range tree.Nodes {
				switch node.Type {
				case "file":
					for _, blob := range node.Content {
						blobs.Insert(BlobHandle{ID: blob, Type: DataBlob})
					}
				}
			}
			lock.Unlock()
		}
		return nil
	})
	return wg.Wait()
}
