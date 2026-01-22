// OIDC Authentication handling

import { getOIDCLoginConfig } from './api.js';
import { storeToken, getToken, isTokenExpired, removeToken } from './jwt.js';

/**
 * Initiate OIDC login flow
 */
async function initiateLogin() {
    try {
        const config = await getOIDCLoginConfig();
        
        // Generate state token for CSRF protection
        const state = generateState();
        sessionStorage.setItem('oidc_state', state);
        
        // Build authorization URL
        const params = new URLSearchParams({
            response_type: 'code',
            client_id: config.data.client_id,
            redirect_uri: config.data.redirect_uri,
            scope: config.data.scope,
            state: state
        });
        
        const authUrl = `${config.data.authorization_endpoint}?${params.toString()}`;
        
        // Debug logging
        console.log('OIDC Config:', config.data);
        console.log('Authorization URL:', authUrl);
        console.log('Parameters:', Object.fromEntries(params));
        
        // Redirect to OIDC provider
        window.location.href = authUrl;
    } catch (error) {
        console.error('Login error:', error);
        const errorMessage = error.message || 'Failed to initiate login. Please try again.';
        showError(errorMessage);
    }
}

/**
 * Handle OIDC callback
 */
async function handleCallback() {
    const urlParams = new URLSearchParams(window.location.search);
    const code = urlParams.get('code');
    const state = urlParams.get('state');
    const error = urlParams.get('error');
    
    if (error) {
        showError(`Authentication error: ${error}`);
        return;
    }
    
    if (!code || !state) {
        return;
    }
    
    // Verify state
    const storedState = sessionStorage.getItem('oidc_state');
    if (state !== storedState) {
        showError('Invalid state parameter. Please try again.');
        return;
    }
    sessionStorage.removeItem('oidc_state');
    
    // Get OIDC config to find token endpoint
    const config = await getOIDCLoginConfig();
    
    // Exchange code for token
    // Note: In a real implementation, this should be done securely
    // For Cognito, we need to call the token endpoint
    try {
        const tokenResponse = await exchangeCodeForToken(code, config.data);
        
        if (tokenResponse.id_token) {
            storeToken(tokenResponse.id_token);
            // Redirect to app
            window.location.href = 'app.html';
        } else {
            showError('Failed to obtain authentication token');
        }
    } catch (error) {
        console.error('Token exchange error:', error);
        showError('Failed to complete authentication. Please try again.');
    }
}

/**
 * Exchange authorization code for tokens
 * Note: This should ideally be done server-side for security
 */
async function exchangeCodeForToken(code, oidcConfig) {
    // Use token endpoint from config if available, otherwise derive from authorization endpoint
    const tokenEndpoint = oidcConfig.token_endpoint || oidcConfig.authorization_endpoint.replace('/oauth2/authorize', '/oauth2/token');
    
    console.log('Token exchange - endpoint:', tokenEndpoint);
    console.log('Token exchange - params:', {
        grant_type: 'authorization_code',
        code: code.substring(0, 20) + '...', // Log partial code for debugging
        client_id: oidcConfig.client_id,
        redirect_uri: oidcConfig.redirect_uri
    });
    
    const response = await fetch(tokenEndpoint, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded'
        },
        body: new URLSearchParams({
            grant_type: 'authorization_code',
            code: code,
            client_id: oidcConfig.client_id,
            redirect_uri: oidcConfig.redirect_uri
        })
    });
    
    if (!response.ok) {
        const errorText = await response.text();
        let errorMessage = `Token exchange failed (${response.status})`;
        try {
            const errorJson = JSON.parse(errorText);
            if (errorJson.error) {
                errorMessage = `Token exchange failed: ${errorJson.error}`;
                if (errorJson.error_description) {
                    errorMessage += ` - ${errorJson.error_description}`;
                }
            }
        } catch {
            if (errorText) {
                errorMessage += `: ${errorText}`;
            }
        }
        console.error('Token exchange error:', errorMessage, errorText);
        throw new Error(errorMessage);
    }
    
    return response.json();
}

/**
 * Logout
 */
function logout() {
    removeToken();
    window.location.href = 'index.html';
}

/**
 * Check if user is authenticated
 */
function isAuthenticated() {
    const token = getToken();
    if (!token) {
        return false;
    }
    
    if (isTokenExpired(token)) {
        removeToken();
        return false;
    }
    
    return true;
}

/**
 * Generate random state token
 */
function generateState() {
    const array = new Uint8Array(32);
    crypto.getRandomValues(array);
    return Array.from(array, byte => byte.toString(16).padStart(2, '0')).join('');
}

/**
 * Show error message
 */
function showError(message) {
    const errorEl = document.getElementById('error-message');
    if (errorEl) {
        errorEl.textContent = message;
        errorEl.style.display = 'block';
    }
}

// Export functions for ES module use
export { initiateLogin, handleCallback, exchangeCodeForToken, logout, isAuthenticated, generateState, showError };

// Expose functions globally for backward compatibility
if (typeof window !== 'undefined') {
    window.initiateLogin = initiateLogin;
    window.handleCallback = handleCallback;
    window.exchangeCodeForToken = exchangeCodeForToken;
    window.logout = logout;
    window.isAuthenticated = isAuthenticated;
    window.generateState = generateState;
    window.showError = showError;
}
