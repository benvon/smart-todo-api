// JWT token utilities

const JWT_STORAGE_KEY = 'smart_todo_jwt_token';

/**
 * Store JWT token in localStorage
 */
function storeToken(token) {
    localStorage.setItem(JWT_STORAGE_KEY, token);
}

/**
 * Get JWT token from localStorage
 */
function getToken() {
    return localStorage.getItem(JWT_STORAGE_KEY);
}

/**
 * Remove JWT token from localStorage
 */
function removeToken() {
    localStorage.removeItem(JWT_STORAGE_KEY);
}

/**
 * Parse JWT token to check expiration
 * Note: This is a simple base64 decode, not full verification
 */
function parseToken(token) {
    try {
        const parts = token.split('.');
        if (parts.length !== 3) {
            return null;
        }
        
        const payload = JSON.parse(atob(parts[1]));
        return payload;
    } catch (e) {
        console.error('Failed to parse token:', e);
        return null;
    }
}

/**
 * Check if token is expired
 */
function isTokenExpired(token) {
    const payload = parseToken(token);
    if (!payload || !payload.exp) {
        return true;
    }
    
    const exp = payload.exp * 1000; // Convert to milliseconds
    return Date.now() >= exp;
}

/**
 * Get token expiration time
 */
function getTokenExpiration(token) {
    const payload = parseToken(token);
    if (!payload || !payload.exp) {
        return null;
    }
    
    return new Date(payload.exp * 1000);
}

// Export functions for ES module use
export { storeToken, getToken, removeToken, parseToken, isTokenExpired, getTokenExpiration };

// Expose functions globally for backward compatibility
if (typeof window !== 'undefined') {
    window.storeToken = storeToken;
    window.getToken = getToken;
    window.removeToken = removeToken;
    window.parseToken = parseToken;
    window.isTokenExpired = isTokenExpired;
    window.getTokenExpiration = getTokenExpiration;
}
