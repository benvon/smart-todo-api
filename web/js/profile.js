// User Profile Panel functionality

import { getCurrentUser, getAIContext, updateAIContext, getTagStats } from './api.js';
import { logout } from './auth.js';
import logger from './logger.js';
import { showError } from './error-utils.js';
import { escapeHtml } from './html-utils.js';

let currentContext = '';
let isLoadingProfileData = false;

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
        // Use setTimeout to ensure DOM is ready after display change
        setTimeout(() => {
            loadProfileData();
        }, 0);
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
    // Prevent concurrent loads
    if (isLoadingProfileData) {
        logger.log('Profile data load already in progress, skipping...');
        return;
    }
    
    isLoadingProfileData = true;
    
    // Set initial loading states
    const userNameEl = document.getElementById('user-name');
    const userEmailEl = document.getElementById('user-email');
    const contextTextarea = document.getElementById('context-textarea');
    const tagStatsEl = document.getElementById('tag-stats');
    
    if (userNameEl) {
        userNameEl.textContent = 'Loading...';
    }
    if (userEmailEl) {
        userEmailEl.textContent = 'Loading...';
    }
    if (contextTextarea) {
        contextTextarea.value = '';
    }
    if (tagStatsEl) {
        tagStatsEl.innerHTML = '<p class="loading-message">Loading tag statistics...</p>';
    }
    
    try {
        // Load all data in parallel using Promise.allSettled
        const results = await Promise.allSettled([
            getCurrentUser(),
            getAIContext(),
            getTagStats()
        ]);

        // Extract all three results
        const [userResult, contextResult, tagStatsResult] = results;
        
        // Handle user info result
        handleUserResult(userResult, userNameEl, userEmailEl);
        
        // Handle AI context result
        handleContextResult(contextResult, contextTextarea);
        
        // Handle tag stats result
        handleTagStatsResult(tagStatsResult, tagStatsEl);
    } catch (error) {
        logger.error('Unexpected error loading profile data:', error);
        if (userNameEl) {
            userNameEl.textContent = 'Error loading';
        }
        if (userEmailEl) {
            userEmailEl.textContent = 'Error loading';
        }
        if (contextTextarea) {
            contextTextarea.value = '';
        }
        if (tagStatsEl) {
            tagStatsEl.innerHTML = '<p class="loading-message">Error loading tag statistics</p>';
        }
    } finally {
        isLoadingProfileData = false;
    }
}

/**
 * Handle user result from API
 */
function handleUserResult(userResult, userNameEl, userEmailEl) {
    if (!userNameEl || !userEmailEl) {
        logger.warn('User info elements not found');
        return;
    }
    
    if (userResult.status === 'fulfilled') {
        if (userResult.value && userResult.value.data) {
            const user = userResult.value.data;
            // Ensure user is an object before accessing properties
            if (typeof user === 'object' && user !== null) {
                userNameEl.textContent = (user.name && user.name.trim()) ? user.name : 'Not provided';
                userEmailEl.textContent = (user.email && user.email.trim()) ? user.email : 'Not provided';
            } else {
                logger.warn('User data is not an object:', user);
                userNameEl.textContent = 'Not available';
                userEmailEl.textContent = 'Not available';
            }
        } else {
            // Response was successful but data is missing
            logger.warn('User data missing from response:', userResult.value);
            userNameEl.textContent = 'Not available';
            userEmailEl.textContent = 'Not available';
        }
    } else if (userResult.status === 'rejected') {
        logger.error('Failed to load user data:', userResult.reason);
        userNameEl.textContent = 'Error loading';
        userEmailEl.textContent = 'Error loading';
        if (!userResult.reason?.isAuthError) {
            showError(userResult.reason?.message || 'Failed to load user data');
        }
    }
}

/**
 * Handle context result from API
 */
function handleContextResult(contextResult, contextTextarea) {
    if (!contextTextarea) {
        logger.warn('Context textarea element not found');
        return;
    }
    
    if (contextResult.status === 'fulfilled') {
        // Handle context_summary - it can be null, undefined, or empty string
        if (contextResult.value && contextResult.value.data) {
            currentContext = contextResult.value.data.context_summary || '';
        } else {
            currentContext = '';
        }
    } else if (contextResult.status === 'rejected') {
        // Clear context on error to avoid showing stale data
        currentContext = '';
        logger.error('Failed to load AI context:', contextResult.reason);
        if (!contextResult.reason?.isAuthError) {
            showError(contextResult.reason?.message || 'Failed to load AI context');
        }
    }
    
    // Update textarea with current context (whether loaded or cleared on error)
    contextTextarea.value = currentContext;
}

/**
 * Handle tag stats result from API
 */
function handleTagStatsResult(tagStatsResult, tagStatsEl) {
    if (!tagStatsEl) {
        logger.warn('Tag stats element not found');
        return;
    }
    
    if (tagStatsResult.status === 'fulfilled') {
        if (tagStatsResult.value && tagStatsResult.value.data) {
            const stats = tagStatsResult.value.data;
            logger.log('Tag stats data:', stats);
            renderTagStats(tagStatsEl, stats);
        } else {
            logger.warn('Tag stats response missing data:', tagStatsResult.value);
            tagStatsEl.innerHTML = '<p class="loading-message">No tag statistics available</p>';
        }
    } else if (tagStatsResult.status === 'rejected') {
        logger.error('Failed to load tag statistics:', tagStatsResult.reason);
        tagStatsEl.innerHTML = '<p class="loading-message">Failed to load tag statistics</p>';
        if (!tagStatsResult.reason?.isAuthError) {
            // Don't show error toast for tag stats failures - just log and show in UI
            logger.warn('Tag stats error (non-auth):', tagStatsResult.reason?.message);
        }
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
        
        // Update textarea to match saved value (in case server modified it)
        contextTextarea.value = currentContext;
        
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



