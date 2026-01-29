// API client

import { removeToken, getToken, isTokenExpired } from './jwt.js';

/**
 * Handle authentication error (no token or expired token)
 * Redirects to login page if not already there
 */
function handleAuthError() {
    removeToken();
    // Only redirect if we're not already on the login page
    if (!window.location.pathname.includes('index.html') && window.location.pathname !== '/') {
        window.location.href = 'index.html';
    }
}

/**
 * Ensure config is loaded before making API requests.
 * Centralizes CONFIG_LOADED so all apiRequest callers get config automatically.
 */
async function ensureConfig() {
    if (window.CONFIG_LOADED) {
        await window.CONFIG_LOADED;
    }
}

/**
 * Make an authenticated API request.
 * Awaits config load before running; supports optional timeout (ms) for profile fetches.
 */
async function apiRequest(endpoint, options = {}) {
    await ensureConfig();

    const token = getToken();
    if (!token) {
        handleAuthError();
        const authError = new Error('No authentication token found');
        authError.isAuthError = true;
        throw authError;
    }

    // Check if token is expired before making the request
    if (isTokenExpired(token)) {
        handleAuthError();
        const authError = new Error('Session expired. Please log in again.');
        authError.isAuthError = true;
        throw authError;
    }

    // Ensure API_BASE_URL is set
    if (!window.API_BASE_URL) {
        const configError = new Error('API_BASE_URL is not configured. Please check your config.json file.');
        configError.isConfigError = true;
        throw configError;
    }

    const url = `${window.API_BASE_URL}${endpoint}`;
    const headers = {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
        ...options.headers
    };

    const fetchOptions = { ...options, headers };
    if (typeof options.timeout === 'number') {
        fetchOptions.signal = AbortSignal.timeout(options.timeout);
    }

    let response;
    try {
        response = await fetch(url, fetchOptions);
    } catch (error) {
        // Log the actual error for debugging
        // eslint-disable-next-line no-console
        console.error('Fetch error caught:', {
            error,
            name: error?.name,
            message: error?.message,
            url,
            method: options.method || 'GET'
        });
        
        // Handle network errors (failed to fetch, CORS, etc.)
        // Fetch errors are typically TypeErrors with messages like "Failed to fetch" or "NetworkError"
        // But we need to be careful - not all TypeErrors are network errors
        // Common fetch error messages:
        // - "Failed to fetch" - network/CORS error
        // - "NetworkError" - network error
        // - "Load failed" - network error
        // - "Network request failed" - network error
        const errorMessage = error?.message || '';
        const errorName = error?.name || '';
        const isNetworkError = (
            // TypeError with fetch-related messages
            (error instanceof TypeError && (
                errorMessage.includes('fetch') || 
                errorMessage.includes('network') ||
                errorMessage.includes('Failed to') ||
                errorMessage === 'Failed to fetch' ||
                errorMessage.includes('Load failed') ||
                errorMessage.includes('Network request failed')
            )) ||
            // DOMException NetworkError
            (error instanceof DOMException && errorName === 'NetworkError') ||
            // Error with NetworkError name
            errorName === 'NetworkError' ||
            // AbortError (request cancelled) - treat as network issue
            (error instanceof DOMException && errorName === 'AbortError')
        );
        
        if (isNetworkError) {
            const networkError = new Error(`Network error: Unable to reach server at ${window.API_BASE_URL}. Please check your connection and try again.`);
            networkError.isNetworkError = true;
            networkError.originalError = error;
            throw networkError;
        }
        // Re-throw other errors as-is
        throw error;
    }

    if (!response.ok) {
        // Handle unauthorized (401) - trigger re-authentication
        if (response.status === 401) {
            handleAuthError();
            const error = await response.json().catch(() => ({ message: 'Unauthorized' }));
            const authError = new Error(error.message || 'Session expired. Please log in again.');
            authError.isAuthError = true;
            throw authError;
        }
        
        // Handle rate limiting (429)
        if (response.status === 429) {
            const retryAfter = response.headers.get('Retry-After') || '60';
            const error = await response.json().catch(() => ({ message: 'Too Many Requests' }));
            const rateLimitError = new Error(error.message || 'Too Many Requests. Please try again later.');
            rateLimitError.retryAfter = parseInt(retryAfter, 10);
            rateLimitError.status = 429;
            throw rateLimitError;
        }
        
        // Handle request size limit (413)
        if (response.status === 413) {
            const error = await response.json().catch(() => ({ message: 'Request Entity Too Large' }));
            throw new Error(error.message || 'Request is too large. Please reduce the size and try again.');
        }
        
        // Handle other errors
        let errorData;
        try {
            const errorText = await response.text();
            if (errorText) {
                errorData = JSON.parse(errorText);
            } else {
                errorData = { message: `HTTP ${response.status}` };
            }
        } catch {
            // If JSON parsing fails, create a generic error
            errorData = { message: `HTTP ${response.status}: ${response.statusText || 'Unknown error'}` };
        }
        // Error response format: {success: false, error: "error_type", message: "error message"}
        const error = new Error(errorData.message || `HTTP ${response.status}`);
        error.status = response.status;
        throw error;
    }

    // Handle 204 No Content (empty response) - DELETE endpoints typically return this
    if (response.status === 204) {
        return { success: true, data: null };
    }

    // Check if response has content
    const contentType = response.headers.get('content-type');
    if (!contentType || !contentType.includes('application/json')) {
        // No JSON content, return success
        return { success: true, data: null };
    }

    // Try to parse JSON, but handle empty responses gracefully
    const text = await response.text();
    if (!text || text.trim() === '') {
        return { success: true, data: null };
    }

    try {
        return JSON.parse(text);
    } catch {
        // If JSON parsing fails but response was OK, return success
        return { success: true, data: null };
    }
}

/**
 * Get OIDC login configuration
 */
async function getOIDCLoginConfig() {
    await ensureConfig();
    const url = `${window.API_BASE_URL}/api/v1/auth/oidc/login`;
    try {
        const response = await fetch(url);
        if (!response.ok) {
            // Handle rate limiting
            if (response.status === 429) {
                const retryAfter = response.headers.get('Retry-After') || '60';
                throw new Error(`Too many requests. Please wait ${retryAfter} seconds before trying again.`);
            }
            
            const errorText = await response.text();
            let errorMessage = `Failed to get OIDC configuration (${response.status})`;
            try {
                const errorJson = JSON.parse(errorText);
                // Error format: {success: false, error: "error_type", message: "error message"}
                if (errorJson.message) {
                    errorMessage = errorJson.message;
                }
            } catch {
                if (errorText) {
                    errorMessage += `: ${errorText}`;
                }
            }
            throw new Error(errorMessage);
        }
        const result = await response.json();
        // Backend returns {success: true, data: {...}, timestamp: ...}
        // Frontend expects an object with a 'data' property containing the config
        // Return the wrapped response so config.data.client_id works correctly
        if (result.success && result.data) {
            return result;
        }
        throw new Error('Invalid response format from server');
    } catch (error) {
        if (error instanceof TypeError && error.message.includes('fetch')) {
            throw new Error(`Network error: Unable to reach server at ${window.API_BASE_URL}. Make sure the server is running.`);
        }
        throw error;
    }
}

/**
 * Get current user.
 * Optional options.timeout (ms) for profile fetches to avoid hanging (e.g. 10000).
 */
async function getCurrentUser(options = {}) {
    return apiRequest('/api/v1/auth/me', options);
}

/**
 * Get todos
 */
async function getTodos(filters = {}) {
    const params = new URLSearchParams();
    if (filters.time_horizon) {
        params.append('time_horizon', filters.time_horizon);
    }
    if (filters.status) {
        params.append('status', filters.status);
    }
    
    const query = params.toString();
    const endpoint = `/api/v1/todos${query ? '?' + query : ''}`;
    return apiRequest(endpoint);
}

/**
 * Create a todo
 */
async function createTodo(text, dueDate = null) {
    const payload = { text };
    if (dueDate) {
        payload.due_date = dueDate;
    }
    return apiRequest('/api/v1/todos', {
        method: 'POST',
        body: JSON.stringify(payload)
    });
}

/**
 * Update a todo
 */
async function updateTodo(id, updates) {
    // Convert due_date to ISO string if it's a Date object
    if (updates.due_date instanceof Date) {
        updates.due_date = updates.due_date.toISOString();
    }
    return apiRequest(`/api/v1/todos/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(updates)
    });
}

/**
 * Delete a todo
 */
async function deleteTodo(id) {
    return apiRequest(`/api/v1/todos/${id}`, {
        method: 'DELETE'
    });
}

/**
 * Complete a todo
 */
async function completeTodo(id) {
    return apiRequest(`/api/v1/todos/${id}/complete`, {
        method: 'POST'
    });
}

/**
 * Trigger AI analysis/reprocessing for a todo
 */
async function analyzeTodo(id) {
    return apiRequest(`/api/v1/todos/${id}/analyze`, {
        method: 'POST'
    });
}

/**
 * Send a message to the AI chat
 */
async function sendChatMessage(message) {
    return apiRequest('/api/v1/ai/chat/message', {
        method: 'POST',
        body: JSON.stringify({ message })
    });
}

/**
 * Check API health status
 */
async function checkAPIHealth() {
    await ensureConfig();
    try {
        const url = `${window.API_BASE_URL}/healthz`;
        const response = await fetch(url, {
            method: 'GET',
            signal: AbortSignal.timeout(5000) // 5 second timeout
        });
        if (response.ok) {
            const data = await response.json();
            return { status: 'healthy', data };
        }
        return { status: 'unhealthy', error: `HTTP ${response.status}` };
    } catch (error) {
        return { status: 'offline', error: error.message };
    }
}

/**
 * Check extended API health status
 */
async function checkExtendedAPIHealth() {
    await ensureConfig();
    try {
        const url = `${window.API_BASE_URL}/healthz?mode=extended`;
        const response = await fetch(url, {
            method: 'GET',
            signal: AbortSignal.timeout(5000) // 5 second timeout
        });
        if (response.ok) {
            const data = await response.json();
            return { status: data.status || 'healthy', data };
        }
        return { status: 'unhealthy', error: `HTTP ${response.status}` };
    } catch (error) {
        return { status: 'offline', error: error.message };
    }
}

/**
 * Get AI context for current user.
 * Optional options.timeout (ms) for profile fetches (e.g. 10000).
 */
async function getAIContext(options = {}) {
    return apiRequest('/api/v1/ai/context', options);
}

/**
 * Update AI context for current user
 */
async function updateAIContext(contextSummary, preferences = null) {
    const payload = {};
    // Include context_summary if provided (including empty string)
    // Empty string is a valid value, so we include it
    // Note: We always include context_summary if it's explicitly provided (even if empty string)
    // This allows clearing the context by sending an empty string
    if (contextSummary !== undefined && contextSummary !== null) {
        payload.context_summary = contextSummary;
    }
    if (preferences !== null) {
        payload.preferences = preferences;
    }
    
    // Ensure we always send at least one field (backend expects at least one)
    // If both are missing, send empty context_summary to clear it
    if (Object.keys(payload).length === 0) {
        payload.context_summary = '';
    }
    
    return apiRequest('/api/v1/ai/context', {
        method: 'PUT',
        body: JSON.stringify(payload)
    });
}

/**
 * Get tag statistics for current user.
 * Optional options.timeout (ms) for profile fetches (e.g. 10000).
 */
async function getTagStats(options = {}) {
    return apiRequest('/api/v1/todos/tags/stats', options);
}

export {
    ensureConfig,
    handleAuthError,
    apiRequest,
    getOIDCLoginConfig,
    getCurrentUser,
    getTodos,
    createTodo,
    updateTodo,
    deleteTodo,
    completeTodo,
    analyzeTodo,
    sendChatMessage,
    checkAPIHealth,
    checkExtendedAPIHealth,
    getAIContext,
    updateAIContext,
    getTagStats
};
