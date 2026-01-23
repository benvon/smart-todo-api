// Logger utility with debug flag support
// Only logs when debug mode is enabled

/**
 * Check if debug mode is enabled
 * Debug mode can be enabled via:
 * - Browser: localStorage.setItem('debug', 'true') or URL parameter: ?debug=true
 * - Node.js: process.env.DEBUG='true' or process.argv includes '--debug'
 */
function isDebugEnabled() {
    // Node.js environment
    if (typeof process !== 'undefined') {
        if (process.env.DEBUG === 'true' || process.env.DEBUG === '1') {
            return true;
        }
        if (process.argv && process.argv.includes('--debug')) {
            return true;
        }
    }

    // Browser environment - Check localStorage
    if (typeof localStorage !== 'undefined') {
        const stored = localStorage.getItem('debug');
        if (stored === 'true' || stored === '1') {
            return true;
        }
    }

    // Browser environment - Check URL parameter
    if (typeof window !== 'undefined' && window.location) {
        const params = new URLSearchParams(window.location.search);
        if (params.get('debug') === 'true' || params.get('debug') === '1') {
            return true;
        }
    }

    return false;
}

/**
 * Logger object that conditionally logs based on debug flag
 */
const logger = {
    log: (...args) => {
        if (isDebugEnabled()) {
            console.log(...args);
        }
    },
    error: (...args) => {
        if (isDebugEnabled()) {
            console.error(...args);
        }
    },
    warn: (...args) => {
        if (isDebugEnabled()) {
            console.warn(...args);
        }
    },
    info: (...args) => {
        if (isDebugEnabled()) {
            console.info(...args);
        }
    },
    debug: (...args) => {
        if (isDebugEnabled()) {
            console.debug(...args);
        }
    }
};

export default logger;
