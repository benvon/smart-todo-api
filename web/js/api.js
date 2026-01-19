// API client

/**
 * Make an authenticated API request
 */
async function apiRequest(endpoint, options = {}) {
    const token = getToken();
    if (!token) {
        throw new Error('No authentication token found');
    }

    const url = `${window.API_BASE_URL}${endpoint}`;
    const headers = {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
        ...options.headers,
    };

    const response = await fetch(url, {
        ...options,
        headers,
    });

    if (!response.ok) {
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
        const error = await response.json().catch(() => ({ message: `HTTP ${response.status}` }));
        // Error response format: {success: false, error: "error_type", message: "error message"}
        throw new Error(error.message || `HTTP ${response.status}`);
    }

    return response.json();
}

/**
 * Get OIDC login configuration
 */
async function getOIDCLoginConfig() {
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
 * Get current user
 */
async function getCurrentUser() {
    return apiRequest('/api/v1/auth/me');
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
async function createTodo(text) {
    return apiRequest('/api/v1/todos', {
        method: 'POST',
        body: JSON.stringify({ text }),
    });
}

/**
 * Update a todo
 */
async function updateTodo(id, updates) {
    return apiRequest(`/api/v1/todos/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(updates),
    });
}

/**
 * Delete a todo
 */
async function deleteTodo(id) {
    return apiRequest(`/api/v1/todos/${id}`, {
        method: 'DELETE',
    });
}

/**
 * Complete a todo
 */
async function completeTodo(id) {
    return apiRequest(`/api/v1/todos/${id}/complete`, {
        method: 'POST',
    });
}
