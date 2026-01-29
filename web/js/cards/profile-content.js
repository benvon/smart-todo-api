// Profile content builder: given profile data (user, context, tagStats), returns DOM for the profile panel body.
// Does not handle open/close or loading; the panel helper and profile module do that.

import { escapeHtml } from '../html-utils.js';

/**
 * Build the inner content of the profile panel (user info, context textarea, tag stats, logout).
 * @param {{ user?: { name?: string, email?: string }, context?: string, tagStats?: { tag_stats?: Object, tainted?: boolean } }} data
 * @param {{ onSaveContext: () => void, onLogout: () => void }} handlers
 * @returns {DocumentFragment}
 */
export function buildProfileContent(data, handlers) {
    const fragment = document.createDocumentFragment();

    const userSection = document.createElement('div');
    userSection.className = 'profile-section';
    userSection.innerHTML = '<h3>User Information</h3>';
    const userInfo = document.createElement('div');
    userInfo.id = 'user-info';
    userInfo.className = 'user-info';
    const name = data?.user?.name?.trim() ?? 'Not provided';
    const email = data?.user?.email?.trim() ?? 'Not provided';
    userInfo.innerHTML = `<p><strong>Name:</strong> <span id="user-name">${escapeHtml(name)}</span></p><p><strong>Email:</strong> <span id="user-email">${escapeHtml(email)}</span></p>`;
    userSection.appendChild(userInfo);
    fragment.appendChild(userSection);

    const contextSection = document.createElement('div');
    contextSection.className = 'profile-section';
    contextSection.innerHTML = '<h3>AI Context</h3><p class="profile-description">This context is used when classifying todo items to better understand your preferences.</p>';
    const contextTextarea = document.createElement('textarea');
    contextTextarea.id = 'context-textarea';
    contextTextarea.className = 'context-textarea';
    contextTextarea.placeholder = 'Enter your preferences and context here...';
    contextTextarea.value = typeof data?.context === 'string' ? data.context : '';
    contextSection.appendChild(contextTextarea);
    const saveContextBtn = document.createElement('button');
    saveContextBtn.id = 'save-context-button';
    saveContextBtn.className = 'btn btn-primary';
    saveContextBtn.textContent = 'Save Context';
    saveContextBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        handlers.onSaveContext();
    });
    contextSection.appendChild(saveContextBtn);
    fragment.appendChild(contextSection);

    const tagSection = document.createElement('div');
    tagSection.className = 'profile-section';
    tagSection.innerHTML = '<h3>Tag Statistics</h3>';
    const tagStatsEl = document.createElement('div');
    tagStatsEl.id = 'tag-stats';
    tagStatsEl.className = 'tag-stats';
    tagStatsEl.appendChild(renderTagStatsContent(data?.tagStats));
    tagSection.appendChild(tagStatsEl);
    fragment.appendChild(tagSection);

    const logoutSection = document.createElement('div');
    logoutSection.className = 'profile-section';
    const logoutBtn = document.createElement('button');
    logoutBtn.id = 'profile-logout-button';
    logoutBtn.className = 'btn btn-secondary';
    logoutBtn.textContent = 'Logout';
    logoutBtn.addEventListener('click', handlers.onLogout);
    logoutSection.appendChild(logoutBtn);
    fragment.appendChild(logoutSection);

    return fragment;
}

/**
 * @param {{ tag_stats?: Object, tainted?: boolean } | undefined} stats
 * @returns {HTMLElement}
 */
function renderTagStatsContent(stats) {
    const wrapper = document.createElement('div');
    if (!stats?.tag_stats || Object.keys(stats.tag_stats).length === 0) {
        wrapper.innerHTML = '<p class="loading-message">No tags found</p>';
        return wrapper;
    }
    if (stats.tainted) {
        const p = document.createElement('p');
        p.className = 'loading-message';
        p.style.color = '#ffc107';
        p.textContent = 'Tag statistics are being updated...';
        wrapper.appendChild(p);
    }
    const tags = Object.entries(stats.tag_stats).sort((a, b) => b[1].total - a[1].total);
    tags.forEach(([tagName, tagStats]) => {
        const item = document.createElement('div');
        item.className = 'tag-stats-item';
        item.innerHTML = `
            <span class="tag-stats-name">${escapeHtml(tagName)}</span>
            <div class="tag-stats-counts">
                <span class="tag-stats-count"><strong>Total:</strong> ${tagStats.total ?? 0}</span>
                <span class="tag-stats-count ai"><strong>AI:</strong> ${tagStats.ai ?? 0}</span>
                <span class="tag-stats-count user"><strong>User:</strong> ${tagStats.user ?? 0}</span>
            </div>
        `;
        wrapper.appendChild(item);
    });
    return wrapper;
}
