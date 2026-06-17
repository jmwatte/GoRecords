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

// -------------------------------------------------------------------------
// Filter Stack (Stepped Filter Wizard)
// -------------------------------------------------------------------------

/**
 * The ordered filter stack. Each entry is { category, value }.
 * Order determines breadcrumb display.  Empty array = no filters.
 */
export const filterStack = writable([]);

/**
 * Index of the currently highlighted breadcrumb chip (-1 = none).
 * Left/Right arrow keys move this; Enter re-opens the picker for that
 * chip's category.
 */
export const breadcrumbIndex = writable(-1);

/**
 * Which picker overlay is currently open, or null if none.
 * Values: 'genre', 'year', 'artist', 'add', or null.
 * 'add' means the "pick a category to add" chooser is shown.
 */
export const activePicker = writable(null);

/**
 * Push a new filter onto the end of the stack.
 * If an identical filter already exists, it is moved to the end
 * (most recently applied) rather than duplicated.
 */
export function pushFilter(category, value) {
  filterStack.update((stack) => {
    const dup = stack.findIndex(
      (f) => f.category === category && f.value === value,
    );
    if (dup >= 0) {
      const f = stack[dup];
      return [...stack.slice(0, dup), ...stack.slice(dup + 1), f];
    }
    return [...stack, { category, value }];
  });
}

/**
 * Remove the filter at the given index from the stack.
 */
export function popFilter(index) {
  filterStack.update((stack) => {
    if (index < 0 || index >= stack.length) return stack;
    return [...stack.slice(0, index), ...stack.slice(index + 1)];
  });
}

/**
 * Clear the entire filter stack.
 */
export function clearFilters() {
  filterStack.set([]);
}

/**
 * Serialise the filter stack to the JSON payload the Go backend expects:
 * [{ field, op, value }].
 */
export function filtersToPayload(filters) {
  return JSON.stringify(
    filters.map((f) => ({
      field: f.category,
      op: "=",
      value: f.value,
    })),
  );
}

/**
 * Cached facet data keyed by field name.
 * Populated by the backend on data refresh.
 */
export const facetData = writable({});
