import { writable } from "svelte/store";

/**
 * A writable store that captures the most recent keyboard event.
 * Components subscribe to this store to react to specific keybindings
 * without attaching their own window listeners.
 */
export const lastKey = writable(null);

/**
 * A registry of active keys currently held down.
 * Useful for chord-aware bindings (e.g., Ctrl+S, Shift+J).
 */
export const activeKeys = writable(new Set());

/**
 * Human-readable map of key bindings used across the app.
 * Extend this as new features are added.
 */
export const KEY = {
  // Navigation
  ARROW_UP: "ArrowUp",
  ARROW_DOWN: "ArrowDown",
  ARROW_LEFT: "ArrowLeft",
  ARROW_RIGHT: "ArrowRight",
  ENTER: "Enter",
  ESCAPE: "Escape",
  TAB: "Tab",
  SPACE: " ",

  // Layout toggles
  LAYOUT_GRID: "g",
  LAYOUT_LIST: "l",

  // Search / filter
  SEARCH_FOCUS: "/",
  CLEAR_FILTERS: "Escape",

  // Playback
  PLAY_PAUSE: " ",
  NEXT: "n",
  PREV: "p",
  VOLUME_UP: "]",
  VOLUME_DOWN: "[",
  MUTE: "m",

  // App
  TOGGLE_SIDEBAR: "b",
  RANDOM_ALBUM: "r",
  SETTINGS: ",",
  REWIND: "z",
  BACK: "<",
};

/**
 * Initialize the global keyboard listener. Call this once from the root
 * App.svelte onMount. Uses capture phase to intercept before default
 * browser behaviour.
 *
 * @returns {() => void} A cleanup function to remove the listener.
 */
export function initKeyboard() {
  function handleKeyDown(e) {
    // Ignore when user is typing in an input / textarea / contenteditable
    const tag = e.target?.tagName?.toLowerCase();
    const editable = e.target?.isContentEditable;
    if (tag === "input" || tag === "textarea" || tag === "select" || editable) {
      // Still propagate the event, but allow the form field to handle it.
      // The filter/search bar listens for Enter/Escape itself.
      return;
    }

    e.preventDefault();

    // Update stores
    lastKey.set(e);
    activeKeys.update((keys) => {
      keys.add(e.key);
      return new Set(keys);
    });
  }

  function handleKeyUp(e) {
    activeKeys.update((keys) => {
      keys.delete(e.key);
      return new Set(keys);
    });
  }

  function handleBlur() {
    // Clear all keys when the window loses focus to prevent stuck keys.
    activeKeys.set(new Set());
  }

  window.addEventListener("keydown", handleKeyDown, { capture: true });
  window.addEventListener("keyup", handleKeyUp);
  window.addEventListener("blur", handleBlur);

  return () => {
    window.removeEventListener("keydown", handleKeyDown, { capture: true });
    window.removeEventListener("keyup", handleKeyUp);
    window.removeEventListener("blur", handleBlur);
  };
}
