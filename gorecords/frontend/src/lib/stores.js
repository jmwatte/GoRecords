import { writable, derived } from "svelte/store";

/**
 * The display mode for album rending in the crate.
 * - 'text':   Compact text-based list (artist — album)
 * - 'visual': Grid with cover art thumbnails
 */
export const viewMode = writable("visual");

/**
 * The currently selected index in the active list (e.g. the focused album
 * in the crate or the focused track in the album_tracks view).
 */
export const currentIndex = writable(0);

/**
 * The top-level view the user is currently in.
 * - 'crate':         The album grid/list (the main library browser)
 * - 'album_tracks':  The track list for a single selected album
 * - 'settings':      The settings / library management view
 */
export const currentView = writable("crate");

/**
 * When in album_tracks view, which album_folder is being browsed.
 */
export const activeAlbumFolder = writable("");

/**
 * Derived store: true when the user is looking at a single album's tracks.
 */
export const isAlbumTracksView = derived(
  currentView,
  ($v) => $v === "album_tracks",
);

/**
 * Derived store: true when the crate is in visual (cover-art grid) mode.
 */
export const isVisualMode = derived(viewMode, ($v) => $v === "visual");

/**
 * Toggle between 'visual' and 'text' view modes.
 */
export function toggleViewMode() {
  viewMode.update((m) => (m === "visual" ? "text" : "visual"));
}

/**
 * Navigate to the album_tracks view for a given album folder.
 */
export function openAlbum(albumFolder) {
  activeAlbumFolder.set(albumFolder);
  currentView.set("album_tracks");
  currentIndex.set(0);
}

/**
 * Go back to the crate from the album_tracks view.
 */
export function closeAlbum() {
  currentView.set("crate");
  activeAlbumFolder.set("");
  currentIndex.set(0);
}

/**
 * Stored music library root path, persisted across sessions.
 */
export const musicRoot = writable("");

/**
 * Scan progress (0-100). -1 means no scan in progress.
 */
export const scanProgress = writable(-1);

/**
 * Active filter stack. Each filter is { field, operator, value }.
 */
export const activeFilters = writable([]);

/**
 * Add or toggle a facet filter. If a filter on the same field with the same
 * value exists, it is removed. Otherwise it is added.
 */
export function toggleFilter(field, operator, value) {
  activeFilters.update((filters) => {
    const idx = filters.findIndex(
      (f) => f.field === field && f.value === value,
    );
    if (idx >= 0) {
      return [...filters.slice(0, idx), ...filters.slice(idx + 1)];
    }
    return [...filters, { field, operator, value }];
  });
}

/**
 * Clear all active filters.
 */
export function clearFilters() {
  activeFilters.set([]);
}

/**
 * Cached facet data keyed by field name.
 */
export const facetData = writable({});
