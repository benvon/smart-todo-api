// Panel helper: show/hide overlay panels with loading, error, and content states.
// Uses vanilla JS; no external dependency. Works with an existing DOM container.

/**
 * Create a panel controller bound to an existing container and content area.
 * @param {Object} options
 * @param {HTMLElement} options.container - The panel overlay element (e.g. profile-panel)
 * @param {HTMLElement} options.contentElement - The inner element to fill with loading/error/content
 * @param {() => void} [options.onClose] - Called when panel is closed (e.g. backdrop or close button)
 * @returns {{ show: () => void, hide: () => void, setContent: (state: { loading?: boolean, error?: string, content?: HTMLElement | (() => HTMLElement) }) => void }}
 */
export function createPanel({ container, contentElement, onClose }) {
    if (!container || !contentElement) {
        throw new Error('createPanel requires container and contentElement');
    }

    function show() {
        container.style.display = 'flex';
    }

    function hide() {
        container.style.display = 'none';
        if (typeof onClose === 'function') {
            onClose();
        }
    }

    /**
     * Update the content area with loading, error, or success content.
     * @param {{ loading?: boolean, error?: string, content?: HTMLElement | (() => HTMLElement) }} state
     */
    function setContent(state) {
        contentElement.innerHTML = '';
        if (state.loading) {
            const p = document.createElement('p');
            p.className = 'loading-message';
            p.textContent = 'Loading...';
            contentElement.appendChild(p);
            return;
        }
        if (state.error) {
            const p = document.createElement('p');
            p.className = 'loading-message';
            p.style.color = '#c00';
            p.textContent = state.error;
            contentElement.appendChild(p);
            return;
        }
        if (state.content) {
            const node = typeof state.content === 'function' ? state.content() : state.content;
            if (node && typeof node.appendChild === 'function') {
                contentElement.appendChild(node);
            }
        }
    }

    return { show, hide, setContent };
}
