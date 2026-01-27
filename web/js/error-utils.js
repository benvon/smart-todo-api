// Error utility functions for displaying error messages in the UI

/**
 * Show error message in a specific error element or the global error element
 * @param {string} message - The error message to display
 * @param {string|HTMLElement} [elementOrId] - Optional: specific error element ID or element to use.
 *                                              If not provided, uses the global 'error-message' element.
 * @param {number} [duration=5000] - Duration in milliseconds to show the error (0 = don't auto-hide)
 */
function showError(message, elementOrId = null, duration = 5000) {
    let errorEl;
    
    // Determine which error element to use
    if (elementOrId) {
        if (typeof elementOrId === 'string') {
            errorEl = document.getElementById(elementOrId);
        } else if (elementOrId instanceof HTMLElement) {
            errorEl = elementOrId;
        }
    } else {
        // Default to global error-message element
        errorEl = document.getElementById('error-message');
    }
    
    if (errorEl) {
        errorEl.textContent = message;
        errorEl.style.display = 'block';
        
        // Auto-hide after duration if duration > 0
        if (duration > 0) {
            setTimeout(() => {
                errorEl.style.display = 'none';
            }, duration);
        }
    }
}

/**
 * Hide error message in a specific error element or the global error element
 * @param {string|HTMLElement} [elementOrId] - Optional: specific error element ID or element to hide.
 *                                              If not provided, uses the global 'error-message' element.
 */
function hideError(elementOrId = null) {
    let errorEl;
    
    if (elementOrId) {
        if (typeof elementOrId === 'string') {
            errorEl = document.getElementById(elementOrId);
        } else if (elementOrId instanceof HTMLElement) {
            errorEl = elementOrId;
        }
    } else {
        errorEl = document.getElementById('error-message');
    }
    
    if (errorEl) {
        errorEl.style.display = 'none';
    }
}

export { showError, hideError };
