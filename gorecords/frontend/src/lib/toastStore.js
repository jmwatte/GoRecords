import { writable, derived } from 'svelte/store';

/**
 * Toast notification store and helpers.
 * Import showToast / dismissToast from this module in any component.
 */

/** @type {import('svelte/store').Writable<Toast[]>} */
export const toasts = writable([]);

let nextId = 0;

/**
 * Show a toast notification.
 * @param {string} message
 * @param {'info'|'warn'|'error'} [type='info']
 * @param {number} [duration=4000] - ms before auto-dismiss, 0 = sticky
 */
export function showToast(message, type = 'info', duration = 4000) {
  const id = nextId++;
  const toast = { id, message, type, duration };
  toasts.update((t) => [...t, toast]);

  if (duration > 0) {
    setTimeout(() => {
      dismissToast(id);
    }, duration);
  }
}

/**
 * Dismiss a specific toast by id.
 * @param {number} id
 */
export function dismissToast(id) {
  toasts.update((t) => t.filter((toast) => toast.id !== id));
}

/**
 * Derived store: true if any toasts are active.
 */
export const hasToasts = derived(toasts, ($t) => $t.length > 0);
