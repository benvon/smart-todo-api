// Single source of truth for current AI context.
// Used by chat and profile so they stay in sync.

let currentContext = '';

/**
 * Get the current context text (in-memory; does not fetch from API).
 * @returns {string}
 */
export function getContext() {
    return currentContext;
}

/**
 * Set the current context text (in-memory only; does not call API).
 * @param {string} text
 */
export function setContext(text) {
    currentContext = typeof text === 'string' ? text : '';
}
