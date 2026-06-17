package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

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

	// Show debug-level logs (including duration decoder failures)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

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
// The frontend calls this when the user presses R.
func (a *App) GetRandomAlbum(filtersJSON string) string {
	filters := parseFiltersJSON(filtersJSON)

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

// OpenFolder opens the file manager to show the containing folder of the given path.
func (a *App) OpenFolder(path string) error {
	cmd := exec.Command("explorer.exe", "/select,", path)
	return cmd.Start()
}

// PickFolder opens the native OS directory picker dialog and returns the
// selected path, or an empty string if the user cancelled.
// The defaultDirectory parameter sets the initial folder shown in the dialog.
func (a *App) PickFolder(defaultDirectory string) string {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choose Music Folder",
		DefaultDirectory: defaultDirectory,
	})
	if err != nil {
		slog.Warn("directory picker cancelled or failed", "error", err)
		return ""
	}
	return dir
}

// GetAlbums returns all unique albums (grouped by album_folder) from the
// database, with aggregated metadata. Pass an offset and limit for pagination.
func (a *App) GetAlbums(offset, limit int) *query.PaginatedAlbums {
	return a.GetFilteredAlbums("[]", offset, limit, "", "")
}

// GetFilteredAlbums returns paginated albums matching the given filters.
// filtersJSON is a JSON array of { field, op, value } objects.
// sortBy and sortDir control the ordering (e.g. "album", "date_added", "year").
// Pass "" for defaults (album ASC).
func (a *App) GetFilteredAlbums(filtersJSON string, offset, limit int, sortBy, sortDir string) *query.PaginatedAlbums {
	filters := parseFiltersJSON(filtersJSON)

	if sortBy == "" {
		sortBy = "album"
	}
	dir := query.SortAsc
	if sortDir == "DESC" {
		dir = query.SortDesc
	}

	q := query.AlbumQuery{
		Filters: filters,
		SortBy:  sortBy,
		SortDir: dir,
		Offset:  offset,
		Limit:   limit,
	}
	result, err := query.GetAlbumsPaginated(DB, q)
	if err != nil {
		slog.Error("failed to get albums", "error", err)
		return &query.PaginatedAlbums{Albums: []query.AlbumResult{}, Total: 0, Offset: offset, Limit: limit}
	}
	return result
}

// GetFacets returns facet counts (distinct values and their occurrence counts)
// for the given field names, constrained by the current filters (excluding the
// facet's own field so users see what refinements are still available).
// filtersJSON is a JSON array of { field, op, value } objects.
func (a *App) GetFacets(filtersJSON string) map[string][]query.Facet {
	filters := parseFiltersJSON(filtersJSON)
	fields := []string{"genre", "year", "album_artist"}
	result, err := query.GenerateFacets(DB, fields, filters)
	if err != nil {
		slog.Error("failed to get facets", "error", err)
		return map[string][]query.Facet{}
	}
	return result
}

// parseFiltersJSON unmarshals a JSON array of { field, op, value } into a
// []query.Filter slice. Returns an empty slice on any parse error.
func parseFiltersJSON(s string) []query.Filter {
	if s == "" || s == "[]" {
		return []query.Filter{}
	}
	var filters []query.Filter
	if err := json.Unmarshal([]byte(s), &filters); err != nil {
		slog.Warn("failed to parse filters JSON", "json", s, "error", err)
		return []query.Filter{}
	}
	return filters
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
