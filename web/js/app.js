// Main application logic: bootstrap, event wiring, API status, add-todo, and high-level handlers.

import { handleCallback, initiateLogin, isAuthenticated } from './auth.js';
import { checkAPIHealth, checkExtendedAPIHealth, createTodo } from './api.js';
import { parseNaturalDate, extractDateFromText, formatDate } from './dateutils.js';
import { initChat } from './chat.js';
import { initProfile } from './profile.js';
import { loadTodos } from './todo-list.js';
import logger from './logger.js';
import { showError } from './error-utils.js';
import { escapeHtml } from './html-utils.js';

document.addEventListener('DOMContentLoaded', async () => {
    if (window.location.pathname.includes('index.html') || window.location.pathname === '/') {
        const urlParams = new URLSearchParams(window.location.search);
        if (urlParams.get('code')) {
            await handleCallback();
            return;
        }
        const loginButton = document.getElementById('login-button');
        if (loginButton) {
            loginButton.addEventListener('click', initiateLogin);
        }
        return;
    }

    if (!isAuthenticated()) {
        window.location.href = 'index.html';
        return;
    }

    initProfile();

    const todoInput = document.getElementById('todo-input');
    const addButton = document.getElementById('add-todo-button');
    if (addButton) {
        addButton.addEventListener('click', handleAddTodo);
    }
    if (todoInput) {
        todoInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                handleAddTodo();
            }
        });
    }
    const dueDateInput = document.getElementById('due-date-input');
    if (dueDateInput) {
        dueDateInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                handleAddTodo();
            }
        });
    }

    await updateAPIStatus();
    setInterval(updateAPIStatus, 10000);

    initAPIStatusDropdown();
    initChat();

    await loadTodos();
    setInterval(async () => {
        if (document.querySelector('.todo-edit-mode')) {
            return;
        }
        await loadTodos();
    }, 3000);
});

async function updateAPIStatus() {
    const statusIndicator = document.getElementById('api-status-indicator');
    const statusText = document.getElementById('api-status-text');
    if (!statusIndicator || !statusText) {
        return;
    }
    try {
        const health = await checkAPIHealth();
        if (health.status === 'healthy') {
            statusIndicator.className = 'api-status-indicator status-online';
            statusText.textContent = 'API Online';
        } else if (health.status === 'unhealthy') {
            statusIndicator.className = 'api-status-indicator status-warning';
            statusText.textContent = 'API Unhealthy';
        } else {
            statusIndicator.className = 'api-status-indicator status-offline';
            statusText.textContent = 'API Offline';
        }
    } catch (error) {
        if (error.isAuthError) {
            return;
        }
        statusIndicator.className = 'api-status-indicator status-offline';
        statusText.textContent = 'API Offline';
    }
}

function initAPIStatusDropdown() {
    const apiStatus = document.getElementById('api-status');
    const dropdown = document.getElementById('api-status-dropdown');
    let refreshInterval = null;
    if (!apiStatus || !dropdown) {
        return;
    }
    apiStatus.addEventListener('click', (e) => {
        e.stopPropagation();
        const isVisible = dropdown.style.display !== 'none';
        if (isVisible) {
            dropdown.style.display = 'none';
            if (refreshInterval) {
                clearInterval(refreshInterval);
                refreshInterval = null;
            }
        } else {
            dropdown.style.display = 'block';
            updateExtendedStatus();
            refreshInterval = setInterval(updateExtendedStatus, 2000);
        }
    });
    document.addEventListener('click', (e) => {
        if (!apiStatus.contains(e.target)) {
            dropdown.style.display = 'none';
            if (refreshInterval) {
                clearInterval(refreshInterval);
                refreshInterval = null;
            }
        }
    });
    dropdown.addEventListener('click', (e) => e.stopPropagation());
}

async function updateExtendedStatus() {
    const dropdownContent = document.getElementById('api-status-dropdown-content');
    if (!dropdownContent) {
        return;
    }
    try {
        const health = await checkExtendedAPIHealth();
        let html = '';
        html += `<div class="api-status-detail"><strong>Status:</strong> <span class="status-${health.status}">${health.status}</span></div>`;
        if (health.data && health.data.timestamp) {
            html += `<div class="api-status-detail"><strong>Timestamp:</strong> ${new Date(health.data.timestamp).toLocaleString()}</div>`;
        }
        if (health.data && health.data.checks) {
            html += '<div class="api-status-detail"><strong>Checks:</strong></div>';
            html += '<div class="api-status-checks">';
            for (const [checkName, checkStatus] of Object.entries(health.data.checks)) {
                html += `<div class="api-status-check-item">
                    <span class="check-name">${escapeHtml(checkName)}:</span>
                    <span class="check-status status-${checkStatus}">${escapeHtml(checkStatus)}</span>
                </div>`;
            }
            html += '</div>';
        }
        if (health.error) {
            html += `<div class="api-status-detail error"><strong>Error:</strong> ${escapeHtml(health.error)}</div>`;
        }
        dropdownContent.innerHTML = html;
    } catch (error) {
        dropdownContent.innerHTML = `<div class="api-status-detail error">Failed to load extended status: ${escapeHtml(error.message)}</div>`;
    }
}

async function handleAddTodo() {
    const input = document.getElementById('todo-input');
    const dueDateInput = document.getElementById('due-date-input');
    let text = input.value.trim();
    const dueDateText = dueDateInput ? dueDateInput.value.trim() : '';

    if (!text) {
        return;
    }

    const MAX_TODO_LENGTH = 10000;
    if (text.length > MAX_TODO_LENGTH) {
        showError(`Todo text cannot exceed ${MAX_TODO_LENGTH} characters. Please shorten your text.`);
        return;
    }

    let dueDate = null;
    if (dueDateText) {
        dueDate = parseNaturalDate(dueDateText);
        if (!dueDate) {
            showError(`Invalid due date format: "${dueDateText}". Try formats like "tomorrow at 3pm", "next Friday", or "2024-03-15T14:30:00Z"`);
            return;
        }
    } else {
        const { cleanedText, detectedDate } = extractDateFromText(text);
        if (detectedDate) {
            dueDate = detectedDate;
            text = cleanedText;
            if (dueDateInput) {
                dueDateInput.value = formatDate(detectedDate);
            }
        }
    }

    try {
        const response = await createTodo(text, dueDate);
        if (response.data) {
            await loadTodos();
        }
        input.value = '';
        if (dueDateInput) {
            dueDateInput.value = '';
        }
    } catch (error) {
        logger.error('Failed to create todo:', error);
        if (error.isAuthError) {
            return;
        }
        if (error.retryAfter) {
            showError(`Too many requests. Please wait ${error.retryAfter} seconds before trying again.`);
        } else {
            showError(error.message || 'Failed to create todo. Please try again.');
        }
    }
}
