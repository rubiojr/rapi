package backend

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/rubiojr/rapi/internal/debug"
	"github.com/rubiojr/rapi/restic"
)

// RetryBackend retries operations on the backend in case of an error with a
// backoff.
type RetryBackend struct {
	restic.Backend
	MaxTries int
	Report   func(string, error, time.Duration)
}

// statically ensure that RetryBackend implements restic.Backend.
var _ restic.Backend = &RetryBackend{}

// NewRetryBackend wraps be with a backend that retries operations after a
// backoff. report is called with a description and the error, if one occurred.
func NewRetryBackend(be restic.Backend, maxTries int, report func(string, error, time.Duration)) *RetryBackend {
	return &RetryBackend{
		Backend:  be,
		MaxTries: maxTries,
		Report:   report,
	}
}

func (be *RetryBackend) retry(ctx context.Context, msg string, f func() error) error {
	err := backoff.RetryNotify(f,
		backoff.WithContext(backoff.WithMaxRetries(backoff.NewExponentialBackOff(), uint64(be.MaxTries)), ctx),
		func(err error, d time.Duration) {
			if be.Report != nil {
				be.Report(msg, err, d)
			}
		},
	)

	return err
}

// Save stores the data in the backend under the given handle.
func (be *RetryBackend) Save(ctx context.Context, h restic.Handle, rd restic.RewindReader) error {
	return be.retry(ctx, fmt.Sprintf("Save(%v)", h), func() error {
		err := rd.Rewind()
		if err != nil {
			return err
		}

		err = be.Backend.Save(ctx, h, rd)
		if err == nil {
			return nil
		}

		debug.Log("Save(%v) failed with error, removing file: %v", h, err)
		rerr := be.Backend.Remove(ctx, h)
		if rerr != nil {
			debug.Log("Remove(%v) returned error: %v", h, err)
		}

		// return original error
		return err
	})
}

// Load returns a reader that yields the contents of the file at h at the
// given offset. If length is larger than zero, only a portion of the file
// is returned. rd must be closed after use. If an error is returned, the
// ReadCloser must be nil.
func (be *RetryBackend) Load(ctx context.Context, h restic.Handle, length int, offset int64, consumer func(rd io.Reader) error) (err error) {
	return be.retry(ctx, fmt.Sprintf("Load(%v, %v, %v)", h, length, offset),
		func() error {
			return be.Backend.Load(ctx, h, length, offset, consumer)
		})
}

// Stat returns information about the File identified by h.
func (be *RetryBackend) Stat(ctx context.Context, h restic.Handle) (fi restic.FileInfo, err error) {
	err = be.retry(ctx, fmt.Sprintf("Stat(%v)", h),
		func() error {
			var innerError error
			fi, innerError = be.Backend.Stat(ctx, h)

			return innerError
		})
	return fi, err
}

// Remove removes a File with type t and name.
func (be *RetryBackend) Remove(ctx context.Context, h restic.Handle) (err error) {
	return be.retry(ctx, fmt.Sprintf("Remove(%v)", h), func() error {
		return be.Backend.Remove(ctx, h)
	})
}

// Test a boolean value whether a File with the name and type exists.
func (be *RetryBackend) Test(ctx context.Context, h restic.Handle) (exists bool, err error) {
	err = be.retry(ctx, fmt.Sprintf("Test(%v)", h), func() error {
		var innerError error
		exists, innerError = be.Backend.Test(ctx, h)

		return innerError
	})
	return exists, err
}

// List runs fn for each file in the backend which has the type t. When an
// error is returned by the underlying backend, the request is retried. When fn
// returns an error, the operation is aborted and the error is returned to the
// caller.
func (be *RetryBackend) List(ctx context.Context, t restic.FileType, fn func(restic.FileInfo) error) error {
	// create a new context that we can cancel when fn returns an error, so
	// that listing is aborted
	listCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	listed := make(map[string]struct{}) // remember for which files we already ran fn
	var innerErr error                  // remember when fn returned an error, so we can return that to the caller

	err := be.retry(listCtx, fmt.Sprintf("List(%v)", t), func() error {
		return be.Backend.List(ctx, t, func(fi restic.FileInfo) error {
			if _, ok := listed[fi.Name]; ok {
				return nil
			}
			listed[fi.Name] = struct{}{}

			innerErr = fn(fi)
			if innerErr != nil {
				// if fn returned an error, listing is aborted, so we cancel the context
				cancel()
			}
			return innerErr
		})
	})

	// the error fn returned takes precedence
	if innerErr != nil {
		return innerErr
	}

	return err
}
