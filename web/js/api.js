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
        const error = await response.json().catch(() => ({ message: 'Unknown error' }));
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
            const errorText = await response.text();
            let errorMessage = `Failed to get OIDC configuration (${response.status})`;
            try {
                const errorJson = JSON.parse(errorText);
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
        return response.json();
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
