// User Profile Panel functionality

import { getCurrentUser, getAIContext, updateAIContext, getTagStats } from './api.js';
import { logout } from './auth.js';
import logger from './logger.js';

let currentContext = '';

/**
 * Initialize profile panel
 */
export function initProfile() {
    const profileButton = document.getElementById('user-profile-button');
    const profilePanel = document.getElementById('profile-panel');
    const closeButton = document.getElementById('profile-close-button');
    const saveContextButton = document.getElementById('save-context-button');
    const profileLogoutButton = document.getElementById('profile-logout-button');

    if (!profileButton || !profilePanel) {
        return;
    }

    // Open profile panel
    profileButton.addEventListener('click', () => {
        profilePanel.style.display = 'flex';
        loadProfileData();
    });

    // Close profile panel
    if (closeButton) {
        closeButton.addEventListener('click', () => {
            profilePanel.style.display = 'none';
        });
    }

    // Close on backdrop click
    profilePanel.addEventListener('click', (e) => {
        if (e.target === profilePanel) {
            profilePanel.style.display = 'none';
        }
    });
    
    // Prevent clicks inside the panel content from closing the panel
    const profilePanelContent = document.querySelector('.profile-panel-content');
    if (profilePanelContent) {
        profilePanelContent.addEventListener('click', (e) => {
            e.stopPropagation(); // Prevent clicks inside content from bubbling to backdrop
        });
    }

    // Save context
    if (saveContextButton) {
        saveContextButton.addEventListener('click', (e) => {
            e.stopPropagation(); // Prevent event from bubbling to backdrop
            handleSaveContext();
        });
    }

    // Logout from profile panel
    if (profileLogoutButton) {
        profileLogoutButton.addEventListener('click', logout);
    }
}

/**
 * Load profile data (user info, context, tag stats)
 */
async function loadProfileData() {
    try {
        // Load user info
        const userResponse = await getCurrentUser();
        if (userResponse.data) {
            const user = userResponse.data;
            const userNameEl = document.getElementById('user-name');
            const userEmailEl = document.getElementById('user-email');
            if (userNameEl) {
                userNameEl.textContent = user.name || 'Not provided';
            }
            if (userEmailEl) {
                userEmailEl.textContent = user.email || 'Not provided';
            }
        }

        // Load AI context
        const contextResponse = await getAIContext();
        // Handle context_summary - it can be null, undefined, or empty string
        if (contextResponse && contextResponse.data) {
            currentContext = contextResponse.data.context_summary || '';
        } else {
            currentContext = '';
        }
        const contextTextarea = document.getElementById('context-textarea');
        if (contextTextarea) {
            contextTextarea.value = currentContext;
        }

        // Load tag statistics
        await loadTagStats();
    } catch (error) {
        logger.error('Failed to load profile data:', error);
        if (!error.isAuthError) {
            showError(error.message || 'Failed to load profile data');
        }
    }
}

/**
 * Load tag statistics
 */
async function loadTagStats() {
    const tagStatsEl = document.getElementById('tag-stats');
    if (!tagStatsEl) {
        return;
    }

    try {
        const response = await getTagStats();
        logger.log('Tag stats response:', response);
        
        if (response && response.data) {
            const stats = response.data;
            logger.log('Tag stats data:', stats);
            renderTagStats(tagStatsEl, stats);
        } else {
            logger.warn('Tag stats response missing data:', response);
            tagStatsEl.innerHTML = '<p class="loading-message">No tag statistics available</p>';
        }
    } catch (error) {
        logger.error('Failed to load tag statistics:', error);
        tagStatsEl.innerHTML = '<p class="loading-message">Failed to load tag statistics</p>';
    }
}

/**
 * Render tag statistics
 */
function renderTagStats(container, stats) {
    logger.log('Rendering tag stats:', stats);
    
    // Check if tag_stats exists and has data
    if (!stats || !stats.tag_stats || Object.keys(stats.tag_stats).length === 0) {
        container.innerHTML = '<p class="loading-message">No tags found</p>';
        return;
    }

    let html = '';
    if (stats.tainted) {
        html += '<p class="loading-message" style="color: #ffc107;">Tag statistics are being updated...</p>';
    }

    const tags = Object.entries(stats.tag_stats).sort((a, b) => {
        // Sort by total count descending
        return b[1].total - a[1].total;
    });

    tags.forEach(([tagName, tagStats]) => {
        html += `
            <div class="tag-stats-item">
                <span class="tag-stats-name">${escapeHtml(tagName)}</span>
                <div class="tag-stats-counts">
                    <span class="tag-stats-count">
                        <strong>Total:</strong> ${tagStats.total || 0}
                    </span>
                    <span class="tag-stats-count ai">
                        <strong>AI:</strong> ${tagStats.ai || 0}
                    </span>
                    <span class="tag-stats-count user">
                        <strong>User:</strong> ${tagStats.user || 0}
                    </span>
                </div>
            </div>
        `;
    });

    container.innerHTML = html;
}

/**
 * Handle saving context
 */
async function handleSaveContext() {
    const contextTextarea = document.getElementById('context-textarea');
    const saveButton = document.getElementById('save-context-button');
    
    if (!contextTextarea || !saveButton) {
        return;
    }

    const newContext = contextTextarea.value.trim();
    
    // Disable button while saving
    saveButton.disabled = true;
    saveButton.textContent = 'Saving...';

    try {
        // Ensure we send empty string, not null/undefined
        const contextToSave = newContext === '' ? '' : newContext;
        const response = await updateAIContext(contextToSave);
        
        // Update current context from response if available
        if (response && response.data && response.data.context_summary !== undefined) {
            currentContext = response.data.context_summary || '';
        } else {
            currentContext = contextToSave;
        }
        
        // Change button text temporarily to show success
        saveButton.textContent = 'Saved!';
        setTimeout(() => {
            saveButton.textContent = 'Save Context';
        }, 2000);
    } catch (error) {
        logger.error('Failed to save context:', error);
        if (!error.isAuthError) {
            // Show more specific error message
            let errorMsg = 'Failed to save context. Please try again.';
            if (error.message) {
                errorMsg = error.message;
            } else if (error.isNetworkError) {
                errorMsg = `Network error: ${error.message || 'Unable to reach server. Please check your connection and try again.'}`;
            }
            showError(errorMsg);
        }
    } finally {
        saveButton.disabled = false;
    }
}

/**
 * Get current context text
 */
export function getCurrentContextText() {
    return currentContext;
}

/**
 * Escape HTML to prevent XSS
 */
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

/**
 * Show error message (reuse from app.js)
 */
function showError(message) {
    const errorEl = document.getElementById('error-message');
    if (errorEl) {
        errorEl.textContent = message;
        errorEl.style.display = 'block';
        setTimeout(() => {
            errorEl.style.display = 'none';
        }, 5000);
    }
}
