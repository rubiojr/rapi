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
	"golang.org/x/sync/errgroup"
)

// Checker runs various checks on a repository. It is advisable to create an
// exclusive Lock in the repository before running any checks.
//
// A Checker only tests for internal errors within the data structures of the
// repository (e.g. missing blobs), and needs a valid Repository to work on.
type Checker struct {
	packs    restic.IDSet
	blobRefs struct {
		sync.Mutex
		// see flags below
		M map[restic.BlobHandle]blobStatus
	}

	masterIndex *repository.MasterIndex

	repo restic.Repository
}

type blobStatus uint8

const (
	blobStatusExists blobStatus = 1 << iota
	blobStatusReferenced
)

// New returns a new checker which runs on repo.
func New(repo restic.Repository) *Checker {
	c := &Checker{
		packs:       restic.NewIDSet(),
		masterIndex: repository.NewMasterIndex(),
		repo:        repo,
	}

	c.blobRefs.M = make(map[restic.BlobHandle]blobStatus)

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

	// track spawned goroutines using wg, create a new context which is
	// cancelled as soon as an error occurs.
	wg, ctx := errgroup.WithContext(ctx)

	type FileInfo struct {
		restic.ID
		Size int64
	}

	type Result struct {
		*repository.Index
		restic.ID
		Err error
	}

	ch := make(chan FileInfo)
	resultCh := make(chan Result)

	// send list of index files through ch, which is closed afterwards
	wg.Go(func() error {
		defer close(ch)
		return c.repo.List(ctx, restic.IndexFile, func(id restic.ID, size int64) error {
			select {
			case <-ctx.Done():
				return nil
			case ch <- FileInfo{id, size}:
			}
			return nil
		})
	})

	// a worker receives an index ID from ch, loads the index, and sends it to indexCh
	worker := func() error {
		var buf []byte
		for fi := range ch {
			debug.Log("worker got file %v", fi.ID.Str())
			var err error
			var idx *repository.Index
			oldFormat := false

			buf, err = c.repo.LoadAndDecrypt(ctx, buf[:0], restic.IndexFile, fi.ID)
			if err == nil {
				idx, oldFormat, err = repository.DecodeIndex(buf, fi.ID)
			}

			if oldFormat {
				debug.Log("index %v has old format", fi.ID.Str())
				hints = append(hints, ErrOldIndexFormat{fi.ID})
			}

			err = errors.Wrapf(err, "error loading index %v", fi.ID.Str())

			select {
			case resultCh <- Result{idx, fi.ID, err}:
			case <-ctx.Done():
			}
		}
		return nil
	}

	// run workers on ch
	wg.Go(func() error {
		defer close(resultCh)
		return repository.RunWorkers(defaultParallelism, worker)
	})

	// receive decoded indexes
	packToIndex := make(map[restic.ID]restic.IDSet)
	wg.Go(func() error {
		for res := range resultCh {
			debug.Log("process index %v, err %v", res.ID, res.Err)

			if res.Err != nil {
				errs = append(errs, res.Err)
				continue
			}

			c.masterIndex.Insert(res.Index)

			debug.Log("process blobs")
			cnt := 0
			for blob := range res.Index.Each(ctx) {
				c.packs.Insert(blob.PackID)
				h := restic.BlobHandle{ID: blob.ID, Type: blob.Type}
				c.blobRefs.M[h] = blobStatusExists
				cnt++

				if _, ok := packToIndex[blob.PackID]; !ok {
					packToIndex[blob.PackID] = restic.NewIDSet()
				}
				packToIndex[blob.PackID].Insert(res.ID)
			}

			debug.Log("%d blobs processed", cnt)
		}
		return nil
	})

	err := wg.Wait()
	if err != nil {
		errs = append(errs, err)
	}

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

	c.masterIndex.MergeFinalIndexes()

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
	repoPacks := restic.NewIDSet()

	err := c.repo.List(ctx, restic.PackFile, func(id restic.ID, size int64) error {
		repoPacks.Insert(id)
		return nil
	})

	if err != nil {
		errChan <- err
	}

	// orphaned: present in the repo but not in c.packs
	for orphanID := range repoPacks.Sub(c.packs) {
		select {
		case <-ctx.Done():
			return
		case errChan <- PackError{ID: orphanID, Orphaned: true, Err: errors.New("not referenced in any index")}:
		}
	}

	// missing: present in c.packs but not in the repo
	for missingID := range c.packs.Sub(repoPacks) {
		select {
		case <-ctx.Done():
			return
		case errChan <- PackError{ID: missingID, Err: errors.New("does not exist")}:
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

func loadTreeFromSnapshot(ctx context.Context, repo restic.Repository, id restic.ID) (restic.ID, error) {
	sn, err := restic.LoadSnapshot(ctx, repo, id)
	if err != nil {
		debug.Log("error loading snapshot %v: %v", id, err)
		return restic.ID{}, err
	}

	if sn.Tree == nil {
		debug.Log("snapshot %v has no tree", id)
		return restic.ID{}, errors.Errorf("snapshot %v has no tree", id)
	}

	return *sn.Tree, nil
}

// loadSnapshotTreeIDs loads all snapshots from backend and returns the tree IDs.
func loadSnapshotTreeIDs(ctx context.Context, repo restic.Repository) (restic.IDs, []error) {
	var trees struct {
		IDs restic.IDs
		sync.Mutex
	}

	var errs struct {
		errs []error
		sync.Mutex
	}

	// track spawned goroutines using wg, create a new context which is
	// cancelled as soon as an error occurs.
	wg, ctx := errgroup.WithContext(ctx)

	ch := make(chan restic.ID)

	// send list of index files through ch, which is closed afterwards
	wg.Go(func() error {
		defer close(ch)
		return repo.List(ctx, restic.SnapshotFile, func(id restic.ID, size int64) error {
			select {
			case <-ctx.Done():
				return nil
			case ch <- id:
			}
			return nil
		})
	})

	// a worker receives an index ID from ch, loads the snapshot and the tree,
	// and adds the result to errs and trees.
	worker := func() error {
		for id := range ch {
			debug.Log("load snapshot %v", id)

			treeID, err := loadTreeFromSnapshot(ctx, repo, id)
			if err != nil {
				errs.Lock()
				errs.errs = append(errs.errs, err)
				errs.Unlock()
				continue
			}

			debug.Log("snapshot %v has tree %v", id, treeID)
			trees.Lock()
			trees.IDs = append(trees.IDs, treeID)
			trees.Unlock()
		}
		return nil
	}

	for i := 0; i < defaultParallelism; i++ {
		wg.Go(worker)
	}

	err := wg.Wait()
	if err != nil {
		errs.errs = append(errs.errs, err)
	}

	return trees.IDs, errs.errs
}

// TreeError collects several errors that occurred while processing a tree.
type TreeError struct {
	ID     restic.ID
	Errors []error
}

func (e TreeError) Error() string {
	return fmt.Sprintf("tree %v: %v", e.ID.Str(), e.Errors)
}

type treeJob struct {
	restic.ID
	error
	*restic.Tree
}

// loadTreeWorker loads trees from repo and sends them to out.
func loadTreeWorker(ctx context.Context, repo restic.Repository,
	in <-chan restic.ID, out chan<- treeJob,
	wg *sync.WaitGroup) {

	defer func() {
		debug.Log("exiting")
		wg.Done()
	}()

	var (
		inCh  = in
		outCh = out
		job   treeJob
	)

	outCh = nil
	for {
		select {
		case <-ctx.Done():
			return

		case treeID, ok := <-inCh:
			if !ok {
				return
			}
			debug.Log("load tree %v", treeID)

			tree, err := repo.LoadTree(ctx, treeID)
			debug.Log("load tree %v (%v) returned err: %v", tree, treeID, err)
			job = treeJob{ID: treeID, error: err, Tree: tree}
			outCh = out
			inCh = nil

		case outCh <- job:
			debug.Log("sent tree %v", job.ID)
			outCh = nil
			inCh = in
		}
	}
}

// checkTreeWorker checks the trees received and sends out errors to errChan.
func (c *Checker) checkTreeWorker(ctx context.Context, in <-chan treeJob, out chan<- error, wg *sync.WaitGroup) {
	defer func() {
		debug.Log("exiting")
		wg.Done()
	}()

	var (
		inCh      = in
		outCh     = out
		treeError TreeError
	)

	outCh = nil
	for {
		select {
		case <-ctx.Done():
			debug.Log("done channel closed, exiting")
			return

		case job, ok := <-inCh:
			if !ok {
				debug.Log("input channel closed, exiting")
				return
			}

			debug.Log("check tree %v (tree %v, err %v)", job.ID, job.Tree, job.error)

			var errs []error
			if job.error != nil {
				errs = append(errs, job.error)
			} else {
				errs = c.checkTree(job.ID, job.Tree)
			}

			if len(errs) > 0 {
				debug.Log("checked tree %v: %v errors", job.ID, len(errs))
				treeError = TreeError{ID: job.ID, Errors: errs}
				outCh = out
				inCh = nil
			}

		case outCh <- treeError:
			debug.Log("tree %v: sent %d errors", treeError.ID, len(treeError.Errors))
			outCh = nil
			inCh = in
		}
	}
}

func (c *Checker) filterTrees(ctx context.Context, backlog restic.IDs, loaderChan chan<- restic.ID, in <-chan treeJob, out chan<- treeJob) {
	defer func() {
		debug.Log("closing output channels")
		close(loaderChan)
		close(out)
	}()

	var (
		inCh                    = in
		outCh                   = out
		loadCh                  = loaderChan
		job                     treeJob
		nextTreeID              restic.ID
		outstandingLoadTreeJobs = 0
	)

	outCh = nil
	loadCh = nil

	for {
		if loadCh == nil && len(backlog) > 0 {
			// process last added ids first, that is traverse the tree in depth-first order
			ln := len(backlog) - 1
			nextTreeID, backlog = backlog[ln], backlog[:ln]

			// use a separate flag for processed trees to ensure that check still processes trees
			// even when a file references a tree blob
			c.blobRefs.Lock()
			h := restic.BlobHandle{ID: nextTreeID, Type: restic.TreeBlob}
			status := c.blobRefs.M[h]
			c.blobRefs.Unlock()
			if (status & blobStatusReferenced) != 0 {
				continue
			}

			loadCh = loaderChan
		}

		if loadCh == nil && outCh == nil && outstandingLoadTreeJobs == 0 {
			debug.Log("backlog is empty, all channels nil, exiting")
			return
		}

		select {
		case <-ctx.Done():
			return

		case loadCh <- nextTreeID:
			outstandingLoadTreeJobs++
			loadCh = nil
			c.blobRefs.Lock()
			h := restic.BlobHandle{ID: nextTreeID, Type: restic.TreeBlob}
			c.blobRefs.M[h] |= blobStatusReferenced
			c.blobRefs.Unlock()

		case j, ok := <-inCh:
			if !ok {
				debug.Log("input channel closed")
				inCh = nil
				in = nil
				continue
			}

			outstandingLoadTreeJobs--

			debug.Log("input job tree %v", j.ID)

			if j.error != nil {
				debug.Log("received job with error: %v (tree %v, ID %v)", j.error, j.Tree, j.ID)
			} else if j.Tree == nil {
				debug.Log("received job with nil tree pointer: %v (ID %v)", j.error, j.ID)
				// send a new job with the new error instead of the old one
				j = treeJob{ID: j.ID, error: errors.New("tree is nil and error is nil")}
			} else {
				subtrees := j.Tree.Subtrees()
				debug.Log("subtrees for tree %v: %v", j.ID, subtrees)
				// iterate backwards over subtree to compensate backwards traversal order of nextTreeID selection
				for i := len(subtrees) - 1; i >= 0; i-- {
					id := subtrees[i]
					if id.IsNull() {
						// We do not need to raise this error here, it is
						// checked when the tree is checked. Just make sure
						// that we do not add any null IDs to the backlog.
						debug.Log("tree %v has nil subtree", j.ID)
						continue
					}
					backlog = append(backlog, id)
				}
			}

			job = j
			outCh = out
			inCh = nil

		case outCh <- job:
			debug.Log("tree sent to check: %v", job.ID)
			outCh = nil
			inCh = in
		}
	}
}

// Structure checks that for all snapshots all referenced data blobs and
// subtrees are available in the index. errChan is closed after all trees have
// been traversed.
func (c *Checker) Structure(ctx context.Context, errChan chan<- error) {
	defer close(errChan)

	trees, errs := loadSnapshotTreeIDs(ctx, c.repo)
	debug.Log("need to check %d trees from snapshots, %d errs returned", len(trees), len(errs))

	for _, err := range errs {
		select {
		case <-ctx.Done():
			return
		case errChan <- err:
		}
	}

	treeIDChan := make(chan restic.ID)
	treeJobChan1 := make(chan treeJob)
	treeJobChan2 := make(chan treeJob)

	var wg sync.WaitGroup
	for i := 0; i < defaultParallelism; i++ {
		wg.Add(2)
		go loadTreeWorker(ctx, c.repo, treeIDChan, treeJobChan1, &wg)
		go c.checkTreeWorker(ctx, treeJobChan2, errChan, &wg)
	}

	c.filterTrees(ctx, trees, treeIDChan, treeJobChan1, treeJobChan2)

	wg.Wait()
}

func (c *Checker) checkTree(id restic.ID, tree *restic.Tree) (errs []error) {
	debug.Log("checking tree %v", id)

	var blobs []restic.ID

	for _, node := range tree.Nodes {
		switch node.Type {
		case "file":
			if node.Content == nil {
				errs = append(errs, Error{TreeID: id, Err: errors.Errorf("file %q has nil blob list", node.Name)})
			}

			var size uint64
			for b, blobID := range node.Content {
				if blobID.IsNull() {
					errs = append(errs, Error{TreeID: id, Err: errors.Errorf("file %q blob %d has null ID", node.Name, b)})
					continue
				}
				blobs = append(blobs, blobID)
				blobSize, found := c.repo.LookupBlobSize(blobID, restic.DataBlob)
				if !found {
					errs = append(errs, Error{TreeID: id, Err: errors.Errorf("file %q blob %d size could not be found", node.Name, b)})
				}
				size += uint64(blobSize)
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

	for _, blobID := range blobs {
		c.blobRefs.Lock()
		h := restic.BlobHandle{ID: blobID, Type: restic.DataBlob}
		if (c.blobRefs.M[h] & blobStatusExists) == 0 {
			debug.Log("tree %v references blob %v which isn't contained in index", id, blobID)
			errs = append(errs, Error{TreeID: id, BlobID: blobID, Err: errors.New("not found in index")})
		}
		c.blobRefs.M[h] |= blobStatusReferenced
		debug.Log("blob %v is referenced", blobID)
		c.blobRefs.Unlock()
	}

	return errs
}

// UnusedBlobs returns all blobs that have never been referenced.
func (c *Checker) UnusedBlobs() (blobs restic.BlobHandles) {
	c.blobRefs.Lock()
	defer c.blobRefs.Unlock()

	debug.Log("checking %d blobs", len(c.blobRefs.M))
	for id, flags := range c.blobRefs.M {
		if (flags & blobStatusReferenced) == 0 {
			debug.Log("blob %v not referenced", id)
			blobs = append(blobs, id)
		}
	}

	return blobs
}

// CountPacks returns the number of packs in the repository.
func (c *Checker) CountPacks() uint64 {
	return uint64(len(c.packs))
}

// GetPacks returns IDSet of packs in the repository
func (c *Checker) GetPacks() restic.IDSet {
	return c.packs
}

// checkPack reads a pack and checks the integrity of all blobs.
func checkPack(ctx context.Context, r restic.Repository, id restic.ID) error {
	debug.Log("checking pack %v", id)
	h := restic.Handle{Type: restic.PackFile, Name: id.String()}

	packfile, hash, size, err := repository.DownloadAndHash(ctx, r.Backend(), h)
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

	blobs, err := pack.List(r.Key(), packfile, size)
	if err != nil {
		return err
	}

	var errs []error
	var buf []byte
	for i, blob := range blobs {
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
	}

	if len(errs) > 0 {
		return errors.Errorf("pack %v contains %v errors: %v", id.Str(), len(errs), errs)
	}

	return nil
}

// ReadData loads all data from the repository and checks the integrity.
func (c *Checker) ReadData(ctx context.Context, p *restic.Progress, errChan chan<- error) {
	c.ReadPacks(ctx, c.packs, p, errChan)
}

// ReadPacks loads data from specified packs and checks the integrity.
func (c *Checker) ReadPacks(ctx context.Context, packs restic.IDSet, p *restic.Progress, errChan chan<- error) {
	defer close(errChan)

	p.Start()
	defer p.Done()

	g, ctx := errgroup.WithContext(ctx)
	ch := make(chan restic.ID)

	// run workers
	for i := 0; i < defaultParallelism; i++ {
		g.Go(func() error {
			for {
				var id restic.ID
				var ok bool

				select {
				case <-ctx.Done():
					return nil
				case id, ok = <-ch:
					if !ok {
						return nil
					}
				}

				err := checkPack(ctx, c.repo, id)
				p.Report(restic.Stat{Blobs: 1})
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
	for pack := range packs {
		select {
		case ch <- pack:
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
