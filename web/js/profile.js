// User Profile Panel – single load path, panel helper, profile-content builder.
// Depends on api, auth, error-utils, panels/panel, cards/profile-content.
// Context state is updated via shared context module (see context.js).

import { ensureConfig, getCurrentUser, getAIContext, updateAIContext, getTagStats } from './api.js';
import { logout } from './auth.js';
import { setContext } from './context.js';
import { createPanel } from './panels/panel.js';
import { buildProfileContent } from './cards/profile-content.js';
import { showError } from './error-utils.js';

const PROFILE_FETCH_TIMEOUT_MS = 10000;

let profileLoadId = 0;

/**
 * Single function to fetch all profile data (user, context, tag stats) with timeout.
 * Call only after config is ready (caller must await ensureConfig() first).
 * Runs the three requests sequentially so responses cannot interleave or overwrite.
 * @returns {Promise<{ user?: object, context?: string, tagStats?: object }>}
 */
async function fetchProfileData() {
    const timeout = PROFILE_FETCH_TIMEOUT_MS;
    const opts = { timeout };

    let userFromMe = null;
    let contextFromApi = '';
    let tagStatsFromApi = null;

    try {
        const userRes = await getCurrentUser(opts);
        if (userRes?.data !== null && userRes?.data !== undefined) {
            userFromMe = userRes.data;
        }
    } catch (err) {
        if (!err?.isAuthError) {
            showError(err?.message ?? 'Failed to load user data');
        }
    }

    try {
        const contextRes = await getAIContext(opts);
        if (contextRes?.data !== null && contextRes?.data !== undefined) {
            const summary = contextRes.data.context_summary;
            contextFromApi = typeof summary === 'string' ? summary : '';
        }
    } catch (err) {
        if (!err?.isAuthError) {
            showError(err?.message ?? 'Failed to load AI context');
        }
    }

    try {
        const tagStatsRes = await getTagStats(opts);
        if (tagStatsRes?.data !== null && tagStatsRes?.data !== undefined) {
            tagStatsFromApi = tagStatsRes.data;
        }
    } catch (err) {
        if (!err?.isAuthError) {
            showError(err?.message ?? 'Failed to load tag statistics');
        }
    }

    return {
        user: userFromMe,
        context: contextFromApi,
        tagStats: tagStatsFromApi
    };
}

/**
 * Initialize profile panel: wire open/close and use panel + profile-content builder.
 */
export function initProfile() {
    const profileButton = document.getElementById('user-profile-button');
    const profilePanel = document.getElementById('profile-panel');
    const contentElement = document.getElementById('profile-panel-body');
    const closeButton = document.getElementById('profile-close-button');

    if (!profileButton || !profilePanel || !contentElement) {
        return;
    }

    const panel = createPanel({
        container: profilePanel,
        contentElement,
        onClose: () => {}
    });

    profileButton.addEventListener('click', () => {
        panel.show();
        loadProfileData(panel);
    });

    if (closeButton) {
        closeButton.addEventListener('click', () => panel.hide());
    }

    profilePanel.addEventListener('click', (e) => {
        if (e.target === profilePanel) {
            panel.hide();
        }
    });

    const profilePanelContent = profilePanel.querySelector('.profile-panel-content');
    if (profilePanelContent) {
        profilePanelContent.addEventListener('click', (e) => e.stopPropagation());
    }
}

/**
 * Load profile data and render into panel (loading → content or error).
 * Awaits config once before starting requests; only applies result if this load is still the latest.
 * @param {{ setContent: (state: { loading?: boolean, error?: string, content?: HTMLElement | (() => HTMLElement) }) => void }} panel
 */
async function loadProfileData(panel) {
    profileLoadId += 1;
    const thisLoadId = profileLoadId;
    panel.setContent({ loading: true });

    try {
        await ensureConfig();
        if (thisLoadId !== profileLoadId) {
            return;
        }
        const data = await fetchProfileData();
        if (thisLoadId !== profileLoadId) {
            return;
        }
        setContext(typeof data.context === 'string' ? data.context : '');
        panel.setContent({
            content: () => buildProfileContent(data, {
                onSaveContext: () => handleSaveContext(),
                onLogout: logout
            })
        });
    } catch (err) {
        if (thisLoadId !== profileLoadId) {
            return;
        }
        panel.setContent({ error: err?.message ?? 'Failed to load profile' });
        showError(err?.message ?? 'Failed to load profile');
    }
}

/**
 * Read context from panel textarea, save via API, and refresh context in shared module.
 */
async function handleSaveContext() {
    const contextTextarea = document.getElementById('context-textarea');
    const saveButton = document.getElementById('save-context-button');

    if (!contextTextarea || !saveButton) {
        return;
    }

    const newContext = contextTextarea.value.trim();
    saveButton.disabled = true;
    saveButton.textContent = 'Saving...';

    try {
        const response = await updateAIContext(newContext === '' ? '' : newContext);
        const savedContext = response?.data?.context_summary ?? newContext;
        setContext(savedContext);
        contextTextarea.value = savedContext;
        saveButton.textContent = 'Saved!';
        setTimeout(() => {
            saveButton.textContent = 'Save Context';
        }, 2000);
    } catch (error) {
        if (!error.isAuthError) {
            showError(error.message ?? 'Failed to save context. Please try again.');
        }
        saveButton.textContent = 'Save Context';
    } finally {
        saveButton.disabled = false;
    }
}
