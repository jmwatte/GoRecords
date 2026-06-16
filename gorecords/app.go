package main

import (
	"context"
	"fmt"
	"log/slog"

	"gorecords/models"
	"gorecords/query"
	"gorecords/scanner"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	InitDB()
	slog.Info("application started")
}

// GetAlbumTracks returns all tracks for a given album folder, ordered by
// disc number then track number. The frontend uses this to render the
// drill-down track list when a user presses Enter on an album.
func (a *App) GetAlbumTracks(albumFolder string) []*models.Track {
	var tracks []*models.Track
	DB.Where("album_folder = ?", albumFolder).
		Order("disc_number ASC, track_number ASC").
		Find(&tracks)
	return tracks
}

// GetRandomAlbum returns a single random album folder matching the given filters.
// The frontend calls this when the user presses R, then navigates to that album.
func (a *App) GetRandomAlbum(filtersJSON string) string {
	// Parse filters from JSON array
	var filters []query.Filter
	if filtersJSON != "" && filtersJSON != "[]" {
		// For now, accept empty filters — the frontend will send active filters
		// as a JSON array in a future iteration.
	}

	result, err := query.GetRandomAlbum(DB, filters)
	if err != nil {
		slog.Warn("random album query failed", "error", err)
		return ""
	}
	return result.AlbumFolder
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// ScanMusic triggers a full library scan, emitting progress events to the
// frontend via the Wails runtime event system.
func (a *App) ScanMusic(rootDir string) error {
	slog.Info("scan music requested", "rootDir", rootDir)
	emitter := &wailsProgressEmitter{ctx: a.ctx}
	return scanner.FullSync(DB, rootDir, emitter)
}

// wailsProgressEmitter implements scanner.ProgressEmitter using Wails runtime events.
type wailsProgressEmitter struct {
	ctx context.Context
}

func (w *wailsProgressEmitter) EmitProgress(current, total int) {
	runtime.EventsEmit(w.ctx, "scan:progress", map[string]int{
		"current": current,
		"total":   total,
	})
}
