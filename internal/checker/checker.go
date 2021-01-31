package checker

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/rubiojr/rapi/internal/debug"
	"github.com/rubiojr/rapi/internal/errors"
	"github.com/rubiojr/rapi/pack"
	"github.com/rubiojr/rapi/repository"
	"github.com/rubiojr/rapi/restic"
	"github.com/rubiojr/rapi/internal/ui/progress"
	"golang.org/x/sync/errgroup"
)

// Checker runs various checks on a repository. It is advisable to create an
// exclusive Lock in the repository before running any checks.
//
// A Checker only tests for internal errors within the data structures of the
// repository (e.g. missing blobs), and needs a valid Repository to work on.
type Checker struct {
	packs    map[restic.ID]int64
	blobRefs struct {
		sync.Mutex
		M restic.BlobSet
	}
	trackUnused bool

	masterIndex *repository.MasterIndex

	repo restic.Repository
}

// New returns a new checker which runs on repo.
func New(repo restic.Repository, trackUnused bool) *Checker {
	c := &Checker{
		packs:       make(map[restic.ID]int64),
		masterIndex: repository.NewMasterIndex(),
		repo:        repo,
		trackUnused: trackUnused,
	}

	c.blobRefs.M = restic.NewBlobSet()

	return c
}

const defaultParallelism = 5

// ErrDuplicatePacks is returned when a pack is found in more than one index.
type ErrDuplicatePacks struct {
	PackID  restic.ID
	Indexes restic.IDSet
}

func (e ErrDuplicatePacks) Error() string {
	return fmt.Sprintf("pack %v contained in several indexes: %v", e.PackID.Str(), e.Indexes)
}

// ErrOldIndexFormat is returned when an index with the old format is
// found.
type ErrOldIndexFormat struct {
	restic.ID
}

func (err ErrOldIndexFormat) Error() string {
	return fmt.Sprintf("index %v has old format", err.ID.Str())
}

// LoadIndex loads all index files.
func (c *Checker) LoadIndex(ctx context.Context) (hints []error, errs []error) {
	debug.Log("Start")

	packToIndex := make(map[restic.ID]restic.IDSet)
	err := repository.ForAllIndexes(ctx, c.repo, func(id restic.ID, index *repository.Index, oldFormat bool, err error) error {
		debug.Log("process index %v, err %v", id, err)

		if oldFormat {
			debug.Log("index %v has old format", id.Str())
			hints = append(hints, ErrOldIndexFormat{id})
		}

		err = errors.Wrapf(err, "error loading index %v", id.Str())

		if err != nil {
			errs = append(errs, err)
			return nil
		}

		c.masterIndex.Insert(index)

		debug.Log("process blobs")
		cnt := 0
		for blob := range index.Each(ctx) {
			cnt++

			if _, ok := packToIndex[blob.PackID]; !ok {
				packToIndex[blob.PackID] = restic.NewIDSet()
			}
			packToIndex[blob.PackID].Insert(id)
		}

		debug.Log("%d blobs processed", cnt)
		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}

	// Merge index before computing pack sizes, as this needs removed duplicates
	err = c.masterIndex.MergeFinalIndexes()
	if err != nil {
		// abort if an error occurs merging the indexes
		return hints, append(errs, err)
	}

	// compute pack size using index entries
	c.packs = c.masterIndex.PackSize(ctx, false)

	debug.Log("checking for duplicate packs")
	for packID := range c.packs {
		debug.Log("  check pack %v: contained in %d indexes", packID, len(packToIndex[packID]))
		if len(packToIndex[packID]) > 1 {
			hints = append(hints, ErrDuplicatePacks{
				PackID:  packID,
				Indexes: packToIndex[packID],
			})
		}
	}

	err = c.repo.SetIndex(c.masterIndex)
	if err != nil {
		debug.Log("SetIndex returned error: %v", err)
		errs = append(errs, err)
	}

	return hints, errs
}

// PackError describes an error with a specific pack.
type PackError struct {
	ID       restic.ID
	Orphaned bool
	Err      error
}

func (e PackError) Error() string {
	return "pack " + e.ID.Str() + ": " + e.Err.Error()
}

// IsOrphanedPack returns true if the error describes a pack which is not
// contained in any index.
func IsOrphanedPack(err error) bool {
	if e, ok := errors.Cause(err).(PackError); ok && e.Orphaned {
		return true
	}

	return false
}

// Packs checks that all packs referenced in the index are still available and
// there are no packs that aren't in an index. errChan is closed after all
// packs have been checked.
func (c *Checker) Packs(ctx context.Context, errChan chan<- error) {
	defer close(errChan)

	debug.Log("checking for %d packs", len(c.packs))

	debug.Log("listing repository packs")
	repoPacks := make(map[restic.ID]int64)

	err := c.repo.List(ctx, restic.PackFile, func(id restic.ID, size int64) error {
		repoPacks[id] = size
		return nil
	})

	if err != nil {
		errChan <- err
	}

	for id, size := range c.packs {
		reposize, ok := repoPacks[id]
		// remove from repoPacks so we can find orphaned packs
		delete(repoPacks, id)

		// missing: present in c.packs but not in the repo
		if !ok {
			select {
			case <-ctx.Done():
				return
			case errChan <- PackError{ID: id, Err: errors.New("does not exist")}:
			}
			continue
		}

		// size not matching: present in c.packs and in the repo, but sizes do not match
		if size != reposize {
			select {
			case <-ctx.Done():
				return
			case errChan <- PackError{ID: id, Err: errors.Errorf("unexpected file size: got %d, expected %d", reposize, size)}:
			}
		}
	}

	// orphaned: present in the repo but not in c.packs
	for orphanID := range repoPacks {
		select {
		case <-ctx.Done():
			return
		case errChan <- PackError{ID: orphanID, Orphaned: true, Err: errors.New("not referenced in any index")}:
		}
	}
}

// Error is an error that occurred while checking a repository.
type Error struct {
	TreeID restic.ID
	BlobID restic.ID
	Err    error
}

func (e Error) Error() string {
	if !e.BlobID.IsNull() && !e.TreeID.IsNull() {
		msg := "tree " + e.TreeID.Str()
		msg += ", blob " + e.BlobID.Str()
		msg += ": " + e.Err.Error()
		return msg
	}

	if !e.TreeID.IsNull() {
		return "tree " + e.TreeID.Str() + ": " + e.Err.Error()
	}

	return e.Err.Error()
}

// TreeError collects several errors that occurred while processing a tree.
type TreeError struct {
	ID     restic.ID
	Errors []error
}

func (e TreeError) Error() string {
	return fmt.Sprintf("tree %v: %v", e.ID.Str(), e.Errors)
}

// checkTreeWorker checks the trees received and sends out errors to errChan.
func (c *Checker) checkTreeWorker(ctx context.Context, trees <-chan restic.TreeItem, out chan<- error) {
	for job := range trees {
		debug.Log("check tree %v (tree %v, err %v)", job.ID, job.Tree, job.Error)

		var errs []error
		if job.Error != nil {
			errs = append(errs, job.Error)
		} else {
			errs = c.checkTree(job.ID, job.Tree)
		}

		if len(errs) == 0 {
			continue
		}
		treeError := TreeError{ID: job.ID, Errors: errs}
		select {
		case <-ctx.Done():
			return
		case out <- treeError:
			debug.Log("tree %v: sent %d errors", treeError.ID, len(treeError.Errors))
		}
	}
}

func loadSnapshotTreeIDs(ctx context.Context, repo restic.Repository) (ids restic.IDs, errs []error) {
	err := restic.ForAllSnapshots(ctx, repo, nil, func(id restic.ID, sn *restic.Snapshot, err error) error {
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		treeID := *sn.Tree
		debug.Log("snapshot %v has tree %v", id, treeID)
		ids = append(ids, treeID)
		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}

	return ids, errs
}

// Structure checks that for all snapshots all referenced data blobs and
// subtrees are available in the index. errChan is closed after all trees have
// been traversed.
func (c *Checker) Structure(ctx context.Context, p *progress.Counter, errChan chan<- error) {
	trees, errs := loadSnapshotTreeIDs(ctx, c.repo)
	p.SetMax(uint64(len(trees)))
	debug.Log("need to check %d trees from snapshots, %d errs returned", len(trees), len(errs))

	for _, err := range errs {
		select {
		case <-ctx.Done():
			return
		case errChan <- err:
		}
	}

	wg, ctx := errgroup.WithContext(ctx)
	treeStream := restic.StreamTrees(ctx, wg, c.repo, trees, func(treeID restic.ID) bool {
		// blobRefs may be accessed in parallel by checkTree
		c.blobRefs.Lock()
		h := restic.BlobHandle{ID: treeID, Type: restic.TreeBlob}
		blobReferenced := c.blobRefs.M.Has(h)
		// noop if already referenced
		c.blobRefs.M.Insert(h)
		c.blobRefs.Unlock()
		return blobReferenced
	}, p)

	defer close(errChan)
	for i := 0; i < defaultParallelism; i++ {
		wg.Go(func() error {
			c.checkTreeWorker(ctx, treeStream, errChan)
			return nil
		})
	}

	// the wait group should not return an error because no worker returns an
	// error, so panic if that has changed somehow.
	err := wg.Wait()
	if err != nil {
		panic(err)
	}
}

func (c *Checker) checkTree(id restic.ID, tree *restic.Tree) (errs []error) {
	debug.Log("checking tree %v", id)

	for _, node := range tree.Nodes {
		switch node.Type {
		case "file":
			if node.Content == nil {
				errs = append(errs, Error{TreeID: id, Err: errors.Errorf("file %q has nil blob list", node.Name)})
			}

			for b, blobID := range node.Content {
				if blobID.IsNull() {
					errs = append(errs, Error{TreeID: id, Err: errors.Errorf("file %q blob %d has null ID", node.Name, b)})
					continue
				}
				// Note that we do not use the blob size. The "obvious" check
				// whether the sum of the blob sizes matches the file size
				// unfortunately fails in some cases that are not resolveable
				// by users, so we omit this check, see #1887

				_, found := c.repo.LookupBlobSize(blobID, restic.DataBlob)
				if !found {
					debug.Log("tree %v references blob %v which isn't contained in index", id, blobID)
					errs = append(errs, Error{TreeID: id, Err: errors.Errorf("file %q blob %v not found in index", node.Name, blobID)})
				}
			}

			if c.trackUnused {
				// loop a second time to keep the locked section as short as possible
				c.blobRefs.Lock()
				for _, blobID := range node.Content {
					if blobID.IsNull() {
						continue
					}
					h := restic.BlobHandle{ID: blobID, Type: restic.DataBlob}
					c.blobRefs.M.Insert(h)
					debug.Log("blob %v is referenced", blobID)
				}
				c.blobRefs.Unlock()
			}

		case "dir":
			if node.Subtree == nil {
				errs = append(errs, Error{TreeID: id, Err: errors.Errorf("dir node %q has no subtree", node.Name)})
				continue
			}

			if node.Subtree.IsNull() {
				errs = append(errs, Error{TreeID: id, Err: errors.Errorf("dir node %q subtree id is null", node.Name)})
				continue
			}

		case "symlink", "socket", "chardev", "dev", "fifo":
			// nothing to check

		default:
			errs = append(errs, Error{TreeID: id, Err: errors.Errorf("node %q with invalid type %q", node.Name, node.Type)})
		}

		if node.Name == "" {
			errs = append(errs, Error{TreeID: id, Err: errors.New("node with empty name")})
		}
	}

	return errs
}

// UnusedBlobs returns all blobs that have never been referenced.
func (c *Checker) UnusedBlobs(ctx context.Context) (blobs restic.BlobHandles) {
	if !c.trackUnused {
		panic("only works when tracking blob references")
	}
	c.blobRefs.Lock()
	defer c.blobRefs.Unlock()

	debug.Log("checking %d blobs", len(c.blobRefs.M))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for blob := range c.repo.Index().Each(ctx) {
		h := restic.BlobHandle{ID: blob.ID, Type: blob.Type}
		if !c.blobRefs.M.Has(h) {
			debug.Log("blob %v not referenced", h)
			blobs = append(blobs, h)
		}
	}

	return blobs
}

// CountPacks returns the number of packs in the repository.
func (c *Checker) CountPacks() uint64 {
	return uint64(len(c.packs))
}

// GetPacks returns IDSet of packs in the repository
func (c *Checker) GetPacks() map[restic.ID]int64 {
	return c.packs
}

// checkPack reads a pack and checks the integrity of all blobs.
func checkPack(ctx context.Context, r restic.Repository, id restic.ID, size int64) error {
	debug.Log("checking pack %v", id)
	h := restic.Handle{Type: restic.PackFile, Name: id.String()}

	packfile, hash, realSize, err := repository.DownloadAndHash(ctx, r.Backend(), h)
	if err != nil {
		return errors.Wrap(err, "checkPack")
	}

	defer func() {
		_ = packfile.Close()
		_ = os.Remove(packfile.Name())
	}()

	debug.Log("hash for pack %v is %v", id, hash)

	if !hash.Equal(id) {
		debug.Log("Pack ID does not match, want %v, got %v", id, hash)
		return errors.Errorf("Pack ID does not match, want %v, got %v", id.Str(), hash.Str())
	}

	if realSize != size {
		debug.Log("Pack size does not match, want %v, got %v", size, realSize)
		return errors.Errorf("Pack size does not match, want %v, got %v", size, realSize)
	}

	blobs, hdrSize, err := pack.List(r.Key(), packfile, size)
	if err != nil {
		return err
	}

	var errs []error
	var buf []byte
	sizeFromBlobs := uint(hdrSize)
	idx := r.Index()
	for i, blob := range blobs {
		sizeFromBlobs += blob.Length
		debug.Log("  check blob %d: %v", i, blob)

		buf = buf[:cap(buf)]
		if uint(len(buf)) < blob.Length {
			buf = make([]byte, blob.Length)
		}
		buf = buf[:blob.Length]

		_, err := packfile.Seek(int64(blob.Offset), 0)
		if err != nil {
			return errors.Errorf("Seek(%v): %v", blob.Offset, err)
		}

		_, err = io.ReadFull(packfile, buf)
		if err != nil {
			debug.Log("  error loading blob %v: %v", blob.ID, err)
			errs = append(errs, errors.Errorf("blob %v: %v", i, err))
			continue
		}

		nonce, ciphertext := buf[:r.Key().NonceSize()], buf[r.Key().NonceSize():]
		plaintext, err := r.Key().Open(ciphertext[:0], nonce, ciphertext, nil)
		if err != nil {
			debug.Log("  error decrypting blob %v: %v", blob.ID, err)
			errs = append(errs, errors.Errorf("blob %v: %v", i, err))
			continue
		}

		hash := restic.Hash(plaintext)
		if !hash.Equal(blob.ID) {
			debug.Log("  Blob ID does not match, want %v, got %v", blob.ID, hash)
			errs = append(errs, errors.Errorf("Blob ID does not match, want %v, got %v", blob.ID.Str(), hash.Str()))
			continue
		}

		// Check if blob is contained in index and position is correct
		idxHas := false
		for _, pb := range idx.Lookup(blob.BlobHandle) {
			if pb.PackID == id && pb.Offset == blob.Offset && pb.Length == blob.Length {
				idxHas = true
				break
			}
		}
		if !idxHas {
			errs = append(errs, errors.Errorf("Blob %v is not contained in index or position is incorrect", blob.ID.Str()))
			continue
		}
	}

	if int64(sizeFromBlobs) != size {
		debug.Log("Pack size does not match, want %v, got %v", size, sizeFromBlobs)
		errs = append(errs, errors.Errorf("Pack size does not match, want %v, got %v", size, sizeFromBlobs))
	}

	if len(errs) > 0 {
		return errors.Errorf("pack %v contains %v errors: %v", id.Str(), len(errs), errs)
	}

	return nil
}

// ReadData loads all data from the repository and checks the integrity.
func (c *Checker) ReadData(ctx context.Context, errChan chan<- error) {
	c.ReadPacks(ctx, c.packs, nil, errChan)
}

// ReadPacks loads data from specified packs and checks the integrity.
func (c *Checker) ReadPacks(ctx context.Context, packs map[restic.ID]int64, p *progress.Counter, errChan chan<- error) {
	defer close(errChan)

	g, ctx := errgroup.WithContext(ctx)
	type packsize struct {
		id   restic.ID
		size int64
	}
	ch := make(chan packsize)

	// run workers
	for i := 0; i < defaultParallelism; i++ {
		g.Go(func() error {
			for {
				var ps packsize
				var ok bool

				select {
				case <-ctx.Done():
					return nil
				case ps, ok = <-ch:
					if !ok {
						return nil
					}
				}
				err := checkPack(ctx, c.repo, ps.id, ps.size)
				p.Add(1)
				if err == nil {
					continue
				}

				select {
				case <-ctx.Done():
					return nil
				case errChan <- err:
				}
			}
		})
	}

	// push packs to ch
	for pack, size := range packs {
		select {
		case ch <- packsize{id: pack, size: size}:
		case <-ctx.Done():
		}
	}
	close(ch)

	err := g.Wait()
	if err != nil {
		select {
		case <-ctx.Done():
			return
		case errChan <- err:
		}
	}
}
