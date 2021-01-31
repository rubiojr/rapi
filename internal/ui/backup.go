package ui

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/rubiojr/rapi/internal/archiver"
	"github.com/rubiojr/rapi/restic"
	"github.com/rubiojr/rapi/internal/ui/signals"
	"github.com/rubiojr/rapi/internal/ui/termstatus"
)

type counter struct {
	Files, Dirs uint
	Bytes       uint64
}

type fileWorkerMessage struct {
	filename string
	done     bool
}

// Backup reports progress for the `backup` command.
type Backup struct {
	*Message
	*StdioWrapper

	MinUpdatePause time.Duration

	term  *termstatus.Terminal
	start time.Time

	totalBytes uint64

	totalCh     chan counter
	processedCh chan counter
	errCh       chan struct{}
	workerCh    chan fileWorkerMessage
	closed      chan struct{}

	summary struct {
		sync.Mutex
		Files, Dirs struct {
			New       uint
			Changed   uint
			Unchanged uint
		}
		ProcessedBytes uint64
		archiver.ItemStats
	}
}

// NewBackup returns a new backup progress reporter.
func NewBackup(term *termstatus.Terminal, verbosity uint) *Backup {
	return &Backup{
		Message:      NewMessage(term, verbosity),
		StdioWrapper: NewStdioWrapper(term),
		term:         term,
		start:        time.Now(),

		// limit to 60fps by default
		MinUpdatePause: time.Second / 60,

		totalCh:     make(chan counter),
		processedCh: make(chan counter),
		errCh:       make(chan struct{}),
		workerCh:    make(chan fileWorkerMessage),
		closed:      make(chan struct{}),
	}
}

// Run regularly updates the status lines. It should be called in a separate
// goroutine.
func (b *Backup) Run(ctx context.Context) error {
	var (
		lastUpdate       time.Time
		total, processed counter
		errors           uint
		started          bool
		currentFiles     = make(map[string]struct{})
		secondsRemaining uint64
	)

	t := time.NewTicker(time.Second)
	signalsCh := signals.GetProgressChannel()
	defer t.Stop()
	defer close(b.closed)
	// Reset status when finished
	defer func() {
		if b.term.CanUpdateStatus() {
			b.term.SetStatus([]string{""})
		}
	}()

	for {
		forceUpdate := false

		select {
		case <-ctx.Done():
			return nil
		case t, ok := <-b.totalCh:
			if ok {
				total = t
				started = true
			} else {
				// scan has finished
				b.totalCh = nil
				b.totalBytes = total.Bytes
			}
		case s := <-b.processedCh:
			processed.Files += s.Files
			processed.Dirs += s.Dirs
			processed.Bytes += s.Bytes
			started = true
		case <-b.errCh:
			errors++
			started = true
		case m := <-b.workerCh:
			if m.done {
				delete(currentFiles, m.filename)
			} else {
				currentFiles[m.filename] = struct{}{}
			}
		case <-t.C:
			if !started {
				continue
			}

			if b.totalCh == nil {
				secs := float64(time.Since(b.start) / time.Second)
				todo := float64(total.Bytes - processed.Bytes)
				secondsRemaining = uint64(secs / float64(processed.Bytes) * todo)
			}
		case <-signalsCh:
			forceUpdate = true
		}

		// limit update frequency
		if !forceUpdate && (time.Since(lastUpdate) < b.MinUpdatePause || b.MinUpdatePause == 0) {
			continue
		}
		lastUpdate = time.Now()

		b.update(total, processed, errors, currentFiles, secondsRemaining)
	}
}

// update updates the status lines.
func (b *Backup) update(total, processed counter, errors uint, currentFiles map[string]struct{}, secs uint64) {
	var status string
	if total.Files == 0 && total.Dirs == 0 {
		// no total count available yet
		status = fmt.Sprintf("[%s] %v files, %s, %d errors",
			formatDuration(time.Since(b.start)),
			processed.Files, formatBytes(processed.Bytes), errors,
		)
	} else {
		var eta, percent string

		if secs > 0 && processed.Bytes < total.Bytes {
			eta = fmt.Sprintf(" ETA %s", formatSeconds(secs))
			percent = formatPercent(processed.Bytes, total.Bytes)
			percent += "  "
		}

		// include totals
		status = fmt.Sprintf("[%s] %s%v files %s, total %v files %v, %d errors%s",
			formatDuration(time.Since(b.start)),
			percent,
			processed.Files,
			formatBytes(processed.Bytes),
			total.Files,
			formatBytes(total.Bytes),
			errors,
			eta,
		)
	}

	lines := make([]string, 0, len(currentFiles)+1)
	for filename := range currentFiles {
		lines = append(lines, filename)
	}
	sort.Strings(lines)
	lines = append([]string{status}, lines...)

	b.term.SetStatus(lines)
}

// ScannerError is the error callback function for the scanner, it prints the
// error in verbose mode and returns nil.
func (b *Backup) ScannerError(item string, fi os.FileInfo, err error) error {
	b.V("scan: %v\n", err)
	return nil
}

// Error is the error callback function for the archiver, it prints the error and returns nil.
func (b *Backup) Error(item string, fi os.FileInfo, err error) error {
	b.E("error: %v\n", err)
	select {
	case b.errCh <- struct{}{}:
	case <-b.closed:
	}
	return nil
}

// StartFile is called when a file is being processed by a worker.
func (b *Backup) StartFile(filename string) {
	select {
	case b.workerCh <- fileWorkerMessage{filename: filename}:
	case <-b.closed:
	}
}

// CompleteBlob is called for all saved blobs for files.
func (b *Backup) CompleteBlob(filename string, bytes uint64) {
	select {
	case b.processedCh <- counter{Bytes: bytes}:
	case <-b.closed:
	}
}

func formatPercent(numerator uint64, denominator uint64) string {
	if denominator == 0 {
		return ""
	}

	percent := 100.0 * float64(numerator) / float64(denominator)

	if percent > 100 {
		percent = 100
	}

	return fmt.Sprintf("%3.2f%%", percent)
}

func formatSeconds(sec uint64) string {
	hours := sec / 3600
	sec -= hours * 3600
	min := sec / 60
	sec -= min * 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, min, sec)
	}

	return fmt.Sprintf("%d:%02d", min, sec)
}

func formatDuration(d time.Duration) string {
	sec := uint64(d / time.Second)
	return formatSeconds(sec)
}

func formatBytes(c uint64) string {
	b := float64(c)
	switch {
	case c > 1<<40:
		return fmt.Sprintf("%.3f TiB", b/(1<<40))
	case c > 1<<30:
		return fmt.Sprintf("%.3f GiB", b/(1<<30))
	case c > 1<<20:
		return fmt.Sprintf("%.3f MiB", b/(1<<20))
	case c > 1<<10:
		return fmt.Sprintf("%.3f KiB", b/(1<<10))
	default:
		return fmt.Sprintf("%d B", c)
	}
}

// CompleteItem is the status callback function for the archiver when a
// file/dir has been saved successfully.
func (b *Backup) CompleteItem(item string, previous, current *restic.Node, s archiver.ItemStats, d time.Duration) {
	b.summary.Lock()
	b.summary.ItemStats.Add(s)

	// for the last item "/", current is nil
	if current != nil {
		b.summary.ProcessedBytes += current.Size
	}

	b.summary.Unlock()

	if current == nil {
		// error occurred, tell the status display to remove the line
		select {
		case b.workerCh <- fileWorkerMessage{filename: item, done: true}:
		case <-b.closed:
		}
		return
	}

	switch current.Type {
	case "file":
		select {
		case b.processedCh <- counter{Files: 1}:
		case <-b.closed:
		}
		select {
		case b.workerCh <- fileWorkerMessage{filename: item, done: true}:
		case <-b.closed:
		}
	case "dir":
		select {
		case b.processedCh <- counter{Dirs: 1}:
		case <-b.closed:
		}
	}

	if current.Type == "dir" {
		if previous == nil {
			b.VV("new       %v, saved in %.3fs (%v added, %v metadata)", item, d.Seconds(), formatBytes(s.DataSize), formatBytes(s.TreeSize))
			b.summary.Lock()
			b.summary.Dirs.New++
			b.summary.Unlock()
			return
		}

		if previous.Equals(*current) {
			b.VV("unchanged %v", item)
			b.summary.Lock()
			b.summary.Dirs.Unchanged++
			b.summary.Unlock()
		} else {
			b.VV("modified  %v, saved in %.3fs (%v added, %v metadata)", item, d.Seconds(), formatBytes(s.DataSize), formatBytes(s.TreeSize))
			b.summary.Lock()
			b.summary.Dirs.Changed++
			b.summary.Unlock()
		}

	} else if current.Type == "file" {
		select {
		case b.workerCh <- fileWorkerMessage{done: true, filename: item}:
		case <-b.closed:
		}

		if previous == nil {
			b.VV("new       %v, saved in %.3fs (%v added)", item, d.Seconds(), formatBytes(s.DataSize))
			b.summary.Lock()
			b.summary.Files.New++
			b.summary.Unlock()
			return
		}

		if previous.Equals(*current) {
			b.VV("unchanged %v", item)
			b.summary.Lock()
			b.summary.Files.Unchanged++
			b.summary.Unlock()
		} else {
			b.VV("modified  %v, saved in %.3fs (%v added)", item, d.Seconds(), formatBytes(s.DataSize))
			b.summary.Lock()
			b.summary.Files.Changed++
			b.summary.Unlock()
		}
	}
}

// ReportTotal sets the total stats up to now
func (b *Backup) ReportTotal(item string, s archiver.ScanStats) {
	select {
	case b.totalCh <- counter{Files: s.Files, Dirs: s.Dirs, Bytes: s.Bytes}:
	case <-b.closed:
	}

	if item == "" {
		b.V("scan finished in %.3fs: %v files, %s",
			time.Since(b.start).Seconds(),
			s.Files, formatBytes(s.Bytes),
		)
		close(b.totalCh)
		return
	}
}

// Finish prints the finishing messages.
func (b *Backup) Finish(snapshotID restic.ID) {
	// wait for the status update goroutine to shut down
	<-b.closed

	b.P("\n")
	b.P("Files:       %5d new, %5d changed, %5d unmodified\n", b.summary.Files.New, b.summary.Files.Changed, b.summary.Files.Unchanged)
	b.P("Dirs:        %5d new, %5d changed, %5d unmodified\n", b.summary.Dirs.New, b.summary.Dirs.Changed, b.summary.Dirs.Unchanged)
	b.V("Data Blobs:  %5d new\n", b.summary.ItemStats.DataBlobs)
	b.V("Tree Blobs:  %5d new\n", b.summary.ItemStats.TreeBlobs)
	b.P("Added to the repo: %-5s\n", formatBytes(b.summary.ItemStats.DataSize+b.summary.ItemStats.TreeSize))
	b.P("\n")
	b.P("processed %v files, %v in %s",
		b.summary.Files.New+b.summary.Files.Changed+b.summary.Files.Unchanged,
		formatBytes(b.summary.ProcessedBytes),
		formatDuration(time.Since(b.start)),
	)
}

// SetMinUpdatePause sets b.MinUpdatePause. It satisfies the
// ArchiveProgressReporter interface.
func (b *Backup) SetMinUpdatePause(d time.Duration) {
	b.MinUpdatePause = d
}
