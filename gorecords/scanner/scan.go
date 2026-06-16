package scanner

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"gorecords/models"
)

// Common audio file extensions (lowercase).
var audioExts = map[string]bool{
	".mp3":  true,
	".flac": true,
	".ogg":  true,
	".m4a":  true,
	".wav":  true,
	".wma":  true,
	".aac":  true,
	".opus": true,
	".ape":  true,
	".aiff": true,
}

// ScanResult wraps a scanned track or the error that occurred for a file.
type ScanResult struct {
	Track *models.Track
	Err   error
	Path  string
}

// ProgressEmitter is an interface for sending scan progress to the frontend.
// The main package provides an implementation backed by runtime.EventsEmit.
type ProgressEmitter interface {
	// EmitProgress sends a progress update.
	// current is the number of files processed so far, total is the estimated total.
	EmitProgress(current, total int)
}

// Scan walks rootDir recursively, extracts tags concurrently using a worker
// pool, and returns all successfully scanned tracks. Errors for individual
// files are logged and skipped; the overall scan does not fail on per-file errors.
// If emitter is non-nil, progress events are emitted as files are processed.
func Scan(rootDir string, workerCount int, emitter ProgressEmitter) []*models.Track {
	rootDir = filepath.Clean(rootDir)

	if workerCount <= 0 {
		workerCount = runtime.NumCPU()
	}

	slog.Info("starting scan",
		"root", rootDir,
		"workers", workerCount,
	)

	// Channel of file paths to process (buffered to reduce blocking).
	jobs := make(chan string, 1000)
	// Channel of results from workers.
	results := make(chan ScanResult, 1000)

	var wg sync.WaitGroup

	// Start worker pool.
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker(jobs, results, &wg)
	}

	// Walk the directory tree in a separate goroutine.
	var walkWg sync.WaitGroup
	walkWg.Add(1)
	go func() {
		defer walkWg.Done()
		defer close(jobs)

		err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				slog.Warn("walk error, skipping", "path", path, "error", err)
				return nil // skip files we can't access
			}

			if d.IsDir() {
				// Skip hidden directories (starting with ".").
				if strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}

			ext := strings.ToLower(filepath.Ext(d.Name()))
			if !audioExts[ext] {
				return nil
			}

			jobs <- path
			return nil
		})
		if err != nil {
			slog.Error("walk failed", "root", rootDir, "error", err)
		}
	}()

	// Close results channel when all workers are done.
	go func() {
		walkWg.Wait()
		wg.Wait()
		close(results)
	}()

	// Collect results and emit progress.
	var tracks []*models.Track
	processed := 0
	// total is unknown during walk; we use the jobs channel length as an estimate
	// once the walk completes. Start with -1 to indicate unknown.
	estimatedTotal := -1

	for res := range results {
		processed++

		// Once the walk is done, jobs is closed and we know how many were enqueued.
		// We approximate total as the processed count plus remaining in results.
		if estimatedTotal < 0 {
			estimatedTotal = processed + len(results)
		}

		if emitter != nil {
			emitter.EmitProgress(processed, estimatedTotal)
		}

		if res.Err != nil {
			slog.Debug("skipping file", "path", res.Path, "error", res.Err)
			continue
		}
		if res.Track != nil {
			tracks = append(tracks, res.Track)
		}
	}

	// Final 100% event.
	if emitter != nil {
		emitter.EmitProgress(processed, processed)
	}

	slog.Info("scan complete",
		"root", rootDir,
		"tracksFound", len(tracks),
	)
	return tracks
}

// worker pulls file paths from jobs, extracts tags, and sends the result.
func worker(jobs <-chan string, results chan<- ScanResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for path := range jobs {
		track, err := ExtractTags(path)
		results <- ScanResult{
			Track: track,
			Err:   err,
			Path:  path,
		}
	}
}
