// Error utility functions for displaying error messages in the UI

// Map to store timeout IDs for each error element to prevent overlapping timeouts
const errorTimeouts = new WeakMap();

/**
 * Get error element by ID or element reference
 * @param {string|HTMLElement} [elementOrId] - Optional: specific error element ID or element
 * @returns {HTMLElement|null} The error element or null if not found
 */
function getErrorElement(elementOrId = null) {
    if (elementOrId) {
        if (typeof elementOrId === 'string') {
            return document.getElementById(elementOrId);
        } else if (elementOrId instanceof HTMLElement) {
            return elementOrId;
        }
    }
    // Default to global error-message element
    return document.getElementById('error-message');
}

/**
 * Show error message in a specific error element or the global error element
 * @param {string} message - The error message to display
 * @param {string|HTMLElement} [elementOrId] - Optional: specific error element ID or element to use.
 *                                              If not provided, uses the global 'error-message' element.
 * @param {number} [duration=5000] - Duration in milliseconds to show the error (0 = don't auto-hide)
 */
function showError(message, elementOrId = null, duration = 5000) {
    const errorEl = getErrorElement(elementOrId);
    
    if (errorEl) {
        errorEl.textContent = message;
        errorEl.style.display = 'block';
        
        // Clear any existing timeout for this element to prevent premature hiding
        const existingTimeout = errorTimeouts.get(errorEl);
        if (existingTimeout) {
            clearTimeout(existingTimeout);
        }
        
        // Auto-hide after duration if duration > 0
        if (duration > 0) {
            const timeoutId = setTimeout(() => {
                errorEl.style.display = 'none';
                errorTimeouts.delete(errorEl);
            }, duration);
            errorTimeouts.set(errorEl, timeoutId);
        }
    }
}

/**
 * Hide error message in a specific error element or the global error element
 * @param {string|HTMLElement} [elementOrId] - Optional: specific error element ID or element to hide.
 *                                              If not provided, uses the global 'error-message' element.
 */
function hideError(elementOrId = null) {
    const errorEl = getErrorElement(elementOrId);
    
    if (errorEl) {
        // Clear any existing timeout
        const existingTimeout = errorTimeouts.get(errorEl);
        if (existingTimeout) {
            clearTimeout(existingTimeout);
            errorTimeouts.delete(errorEl);
        }
        
        errorEl.style.display = 'none';
    }
}

export { showError, hideError };
