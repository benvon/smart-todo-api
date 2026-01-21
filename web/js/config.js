// API Configuration
// Loads configuration from config.json file
// Fallback to default if config file is not found or fails to load

const DEFAULT_API_BASE_URL = 'http://localhost:8080';

// Set default immediately to avoid undefined errors
window.API_BASE_URL = window.API_BASE_URL || DEFAULT_API_BASE_URL;

// Create a promise that resolves when config is loaded
// This ensures config is loaded before other scripts use window.API_BASE_URL
window.CONFIG_LOADED = (async function loadConfig() {
    try {
        const response = await fetch('/config.json');
        if (!response.ok) {
            console.warn('Config file not found, using default API URL:', DEFAULT_API_BASE_URL);
            return;
        }
        
        const config = await response.json();
        if (config.api_base_url) {
            window.API_BASE_URL = config.api_base_url;
            console.log('Loaded API base URL from config:', window.API_BASE_URL);
        } else {
            console.warn('Config file missing api_base_url, using default:', DEFAULT_API_BASE_URL);
        }
    } catch (error) {
        console.warn('Failed to load config.json, using default API URL:', DEFAULT_API_BASE_URL, error);
    }
})();
