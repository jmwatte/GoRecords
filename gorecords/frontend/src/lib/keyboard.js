import { writable } from "svelte/store";
import keybindings from "./keybindings.json";

/**
 * A writable store that captures the most recent keyboard event.
 * Components subscribe to this store to react to specific keybindings
 * without attaching their own window listeners.
 */
export const lastKey = writable(null);

/**
 * A monotonically increasing counter to guarantee every action dispatch
 * produces a unique store value (so Svelte reactivity always fires).
 */
let actionSeq = 0;

/**
 * Matched action info: { action: string | null, seq: number }.
 * seq increments on every keydown so that pressing the same key repeatedly
 * always triggers Svelte's reactive statements.
 */
export const lastAction = writable({ action: null, seq: 0 });

/**
 * A registry of active keys currently held down.
 * Useful for chord-aware bindings (e.g., Ctrl+S, Shift+J).
 */
export const activeKeys = writable(new Set());

// -------------------------------------------------------------------------
// Exported keybindings list for rendering the help screen
// -------------------------------------------------------------------------
export const BINDINGS = keybindings;

// -------------------------------------------------------------------------
// matchesShortcut(event, shortcut) -> boolean
// -------------------------------------------------------------------------
// Parses a shortcut string like "Shift+G" or "g" or "ArrowUp" or "Ctrl+S"
// and compares it against the given KeyboardEvent.
//
// Rules:
//  - "g"          matches ONLY event.key === 'g' AND no modifiers pressed.
//  - "Shift+G"    matches ONLY event.shiftKey === true AND event.key === 'G'.
//  - "ArrowUp", "Enter", "Escape", "Space"  match by key name, case-sensitive.
//  - "Ctrl+S"     matches event.ctrlKey === true AND event.key === 's'.
// -------------------------------------------------------------------------
export function matchesShortcut(event, shortcut) {
  // Split on "+" to get individual modifier tokens and the base key
  const parts = shortcut.split("+");
  const baseKey = parts[parts.length - 1]; // last token is the base key
  const mods = parts.slice(0, -1); // everything before = modifiers

  // Determine which modifiers are REQUIRED by the shortcut config
  const needsShift = mods.includes("Shift");
  const needsCtrl = mods.includes("Ctrl");
  const needsAlt = mods.includes("Alt");
  const needsMeta = mods.includes("Meta");

  // Check modifier states on the event
  // If a modifier is NOT in the shortcut, it must be NOT pressed
  // (e.g., shortcut "g" should NOT fire when Shift or Ctrl is held)
  if (event.shiftKey !== needsShift) return false;
  if (event.ctrlKey !== needsCtrl) return false;
  if (event.altKey !== needsAlt) return false;
  if (event.metaKey !== needsMeta) return false;

  // Check the base key
  // Named keys (ArrowUp, Enter, Escape, Backspace, Tab) compare exactly by event.key
  if (/^Arrow|^Enter$|^Escape$|^Backspace$|^Tab$/.test(baseKey)) {
    return event.key === baseKey;
  }
  // Space is special: event.key is " " but the shortcut string is "Space"
  if (baseKey === "Space") {
    return event.key === " ";
  }
  // Whitespace characters (" ", "\t", etc.) match directly
  if (/^\s$/.test(baseKey)) {
    return event.key === baseKey;
  }

  // For single-character printable keys, compare case-sensitively.
  // "g" matches event.key === 'g' (lowercase, no shift).
  // "Shift+G" has needsShift=true, so the shift check above ensures event.key is 'G'.
  if (baseKey.length === 1 && baseKey === event.key) {
    return true;
  }

  return false;
}

// -------------------------------------------------------------------------
// Look up which action (if any) matches the given event
// -------------------------------------------------------------------------
export function getAction(event) {
  for (const binding of keybindings) {
    if (matchesShortcut(event, binding.shortcut)) {
      return binding.action;
    }
  }
  return null;
}

// -------------------------------------------------------------------------
// Render a shortcut string as an array of HTML-safe label parts.
// Returns [{ text: "Shift", kbd: true }, { text: " + " }, { text: "G", kbd: true }]
// for use in rendering <kbd> elements.
// -------------------------------------------------------------------------
export function renderShortcut(shortcut) {
  const parts = shortcut.split("+");
  const result = [];
  for (let i = 0; i < parts.length; i++) {
    if (i > 0) {
      result.push({ text: " + ", kbd: false });
    }
    result.push({ text: parts[i], kbd: true });
  }
  return result;
}

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
      return;
    }

    // Find which action (if any) this event matches
    const action = getAction(e);

    // Update stores — the raw event is still available for custom checks
    lastKey.set(e);
    lastAction.set({ action, seq: ++actionSeq });
    activeKeys.update((keys) => {
      keys.add(e.key);
      return new Set(keys);
    });

    // If a binding matched, prevent default behaviour
    if (action) {
      e.preventDefault();
    }
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
