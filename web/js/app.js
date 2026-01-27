// Main application logic

import { handleCallback, initiateLogin, isAuthenticated } from './auth.js';
import { checkAPIHealth, checkExtendedAPIHealth, getTodos, createTodo, updateTodo, deleteTodo, completeTodo, analyzeTodo } from './api.js';
import { parseNaturalDate, extractDateFromText, formatDate } from './dateutils.js';
import { initChat } from './chat.js';
import { initProfile } from './profile.js';
import logger from './logger.js';
import { showError } from './error-utils.js';

let todos = [];

// Initialize app
document.addEventListener('DOMContentLoaded', async () => {
    // Check if we're on the login page
    if (window.location.pathname.includes('index.html') || window.location.pathname === '/') {
        // Check for OIDC callback
        const urlParams = new URLSearchParams(window.location.search);
        if (urlParams.get('code')) {
            await handleCallback();
            return;
        }
        
        // Setup login button
        const loginButton = document.getElementById('login-button');
        if (loginButton) {
            loginButton.addEventListener('click', initiateLogin);
        }
        return;
    }
    
    // App page - check authentication
    if (!isAuthenticated()) {
        window.location.href = 'index.html';
        return;
    }
    
    // Initialize profile panel
    initProfile();
    
    // Setup todo input
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
    
    // Allow Enter key in due date input to submit
    const dueDateInput = document.getElementById('due-date-input');
    if (dueDateInput) {
        dueDateInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                handleAddTodo();
            }
        });
    }
    
    // Initialize API status indicator
    await updateAPIStatus();
    setInterval(updateAPIStatus, 10000); // Update every 10 seconds
    
    // Initialize API status dropdown
    initAPIStatusDropdown();
    
    // Initialize chat
    initChat();
    
    // Load todos
    await loadTodos();
    
    // Start auto-refresh to show status updates
    // Refresh every 3 seconds to catch processing status changes
    setInterval(async () => {
        await loadTodos();
    }, 3000);
});

/**
 * Update API connection status indicator
 */
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
        // Don't update status if it's an auth error (we're redirecting)
        if (error.isAuthError) {
            return;
        }
        statusIndicator.className = 'api-status-indicator status-offline';
        statusText.textContent = 'API Offline';
    }
}

/**
 * Initialize API status dropdown
 */
function initAPIStatusDropdown() {
    const apiStatus = document.getElementById('api-status');
    const dropdown = document.getElementById('api-status-dropdown');
    let refreshInterval = null;
    
    if (!apiStatus || !dropdown) {
        return;
    }
    
    // Toggle dropdown on click
    apiStatus.addEventListener('click', (e) => {
        e.stopPropagation();
        const isVisible = dropdown.style.display !== 'none';
        
        if (isVisible) {
            // Close dropdown
            dropdown.style.display = 'none';
            if (refreshInterval) {
                clearInterval(refreshInterval);
                refreshInterval = null;
            }
        } else {
            // Open dropdown
            dropdown.style.display = 'block';
            updateExtendedStatus();
            // Refresh every 2 seconds when open
            refreshInterval = setInterval(updateExtendedStatus, 2000);
        }
    });
    
    // Close dropdown when clicking outside
    document.addEventListener('click', (e) => {
        if (!apiStatus.contains(e.target)) {
            dropdown.style.display = 'none';
            if (refreshInterval) {
                clearInterval(refreshInterval);
                refreshInterval = null;
            }
        }
    });
    
    // Prevent dropdown clicks from closing it
    dropdown.addEventListener('click', (e) => {
        e.stopPropagation();
    });
}

/**
 * Update extended API status in dropdown
 */
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

/**
 * Escape HTML to prevent XSS
 */
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

/**
 * Load todos from API
 */
async function loadTodos() {
    try {
        const response = await getTodos();
        // Handle paginated response: response.data.todos contains the array
        if (response.data && Array.isArray(response.data.todos)) {
            todos = response.data.todos;
        } else if (Array.isArray(response.data)) {
            // Fallback for non-paginated response (shouldn't happen with new API)
            todos = response.data;
        } else {
            todos = [];
        }
        renderTodos();
    } catch (error) {
        logger.error('Failed to load todos:', error);
        // Don't show error messages for auth errors (we're redirecting)
        if (error.isAuthError) {
            return;
        }
        // Handle rate limiting
        if (error.retryAfter) {
            showError(`Too many requests. Please wait ${error.retryAfter} seconds before trying again.`);
        } else {
            showError(error.message || 'Failed to load todos. Please refresh the page.');
        }
    }
}

/**
 * Handle adding a new todo
 */
async function handleAddTodo() {
    const input = document.getElementById('todo-input');
    const dueDateInput = document.getElementById('due-date-input');
    let text = input.value.trim();
    const dueDateText = dueDateInput ? dueDateInput.value.trim() : '';
    
    if (!text) {
        return;
    }
    
    // Client-side validation: max 10,000 characters (matches server validation)
    const MAX_TODO_LENGTH = 10000;
    if (text.length > MAX_TODO_LENGTH) {
        showError(`Todo text cannot exceed ${MAX_TODO_LENGTH} characters. Please shorten your text.`);
        return;
    }
    
    // Extract date from text if not explicitly provided in due date input
    let dueDate = null;
    if (dueDateText) {
        // Explicit due date input takes precedence
        dueDate = parseNaturalDate(dueDateText);
        if (!dueDate) {
            showError(`Invalid due date format: "${dueDateText}". Try formats like "tomorrow at 3pm", "next Friday", or "2024-03-15T14:30:00Z"`);
            return;
        }
    } else {
        // Try to extract date from the todo text itself
        const { cleanedText, detectedDate } = extractDateFromText(text);
        if (detectedDate) {
            dueDate = detectedDate;
            text = cleanedText; // Use cleaned text without the date expression
            
            // Update the UI to show the detected date in the due date input field
            if (dueDateInput) {
                // Show formatted date in the due date input so user can see what was detected
                const dateDisplay = formatDate(detectedDate);
                dueDateInput.value = dateDisplay;
            }
        }
    }
    
    try {
        const response = await createTodo(text, dueDate);
        // Response format: {success: true, data: {...}, timestamp: ...}
        if (response.data) {
            todos.push(response.data);
        }
        input.value = '';
        if (dueDateInput) {
            dueDateInput.value = '';
        }
        renderTodos();
    } catch (error) {
        logger.error('Failed to create todo:', error);
        // Don't show error messages for auth errors (we're redirecting)
        if (error.isAuthError) {
            return;
        }
        // Handle rate limiting with retry-after info
        if (error.retryAfter) {
            showError(`Too many requests. Please wait ${error.retryAfter} seconds before trying again.`);
        } else {
            showError(error.message || 'Failed to create todo. Please try again.');
        }
    }
}

/**
 * Handle completing a todo
 */
async function handleCompleteTodo(id) {
    try {
        const response = await completeTodo(id);
        // Response format: {success: true, data: {...}, timestamp: ...}
        const updatedTodo = response.data;
        
        if (updatedTodo) {
            // Update in local array
            const index = todos.findIndex(t => t.id === id);
            if (index !== -1) {
                todos[index] = updatedTodo;
            }
        }
        
        renderTodos();
    } catch (error) {
        logger.error('Failed to complete todo:', error);
        // Don't show error messages for auth errors (we're redirecting)
        if (error.isAuthError) {
            return;
        }
        if (error.retryAfter) {
            showError(`Too many requests. Please wait ${error.retryAfter} seconds before trying again.`);
        } else {
            showError(error.message || 'Failed to complete todo. Please try again.');
        }
    }
}

/**
 * Handle deleting a todo
 */
async function handleDeleteTodo(id) {
    if (!confirm('Are you sure you want to delete this todo?')) {
        return;
    }
    
    try {
        await deleteTodo(id);
        
        // Remove from local array
        todos = todos.filter(t => t.id !== id);
        renderTodos();
    } catch (error) {
        logger.error('Failed to delete todo:', error);
        // Don't show error messages for auth errors (we're redirecting)
        if (error.isAuthError) {
            return;
        }
        if (error.retryAfter) {
            showError(`Too many requests. Please wait ${error.retryAfter} seconds before trying again.`);
        } else {
            showError(error.message || 'Failed to delete todo. Please try again.');
        }
    }
}

/**
 * Handle reprocessing a todo (trigger AI analysis)
 */
async function handleReprocessTodo(id, statusBadge) {
    // Don't allow reprocessing if already processing (check data attribute)
    if (statusBadge.dataset.processing === 'true') {
        showError('Todo is already being processed. Please wait.');
        return;
    }
    
    // Don't allow reprocessing if already processing (check todo status)
    const todo = todos.find(t => t.id === id);
    if (todo && todo.status === 'processing') {
        showError('Todo is already being processed. Please wait.');
        return;
    }
    
    // Mark as processing to prevent double-clicks
    statusBadge.dataset.processing = 'true';
    
    // Visual feedback: show processing state immediately
    const originalText = statusBadge.textContent.trim();
    const originalClass = statusBadge.className;
    statusBadge.textContent = 'Processing...';
    statusBadge.className = 'status-badge status-processing status-badge-clickable';
    statusBadge.setAttribute('title', 'Processing...');
    
    // Remove any existing spinner first
    const existingSpinner = statusBadge.querySelector('.spinner');
    if (existingSpinner) {
        existingSpinner.remove();
    }
    
    // Add spinner
    const spinner = document.createElement('span');
    spinner.className = 'spinner';
    statusBadge.appendChild(spinner);
    
    try {
        await analyzeTodo(id);
        // Show success message (note: showError is used for all messages, success or error)
        showError('Reprocessing started. The todo will be updated shortly.');
        // Reload todos after a short delay to show the updated status
        setTimeout(() => {
            loadTodos();
        }, 1000);
    } catch (error) {
        logger.error('Failed to reprocess todo:', error);
        // Restore original state on error
        statusBadge.textContent = originalText;
        statusBadge.className = originalClass;
        statusBadge.setAttribute('title', 'Click to reprocess this todo');
        statusBadge.dataset.processing = 'false';
        // Remove spinner
        const spinnerToRemove = statusBadge.querySelector('.spinner');
        if (spinnerToRemove) {
            spinnerToRemove.remove();
        }
        
        // Don't show error messages for auth errors (we're redirecting)
        if (error.isAuthError) {
            return;
        }
        if (error.retryAfter) {
            showError(`Too many requests. Please wait ${error.retryAfter} seconds before trying again.`);
        } else {
            showError(error.message || 'Failed to reprocess todo. Please try again.');
        }
    }
}

/**
 * Render todos in columns
 */
function renderTodos() {
    const nextList = document.getElementById('todos-next');
    const soonList = document.getElementById('todos-soon');
    const laterList = document.getElementById('todos-later');
    
    // Collect todos currently in edit mode to preserve them during re-render
    const editModeTodos = document.querySelectorAll('.todo-item.todo-edit-mode');
    const editModeElements = new Map();
    editModeTodos.forEach(el => {
        const todoId = el.getAttribute('data-todo-id');
        if (todoId) {
            // Store with string key to match the lookup below
            editModeElements.set(String(todoId), el);
        }
    });
    
    // Clear existing todos
    [nextList, soonList, laterList].forEach(list => {
        if (list) {
            list.innerHTML = '';
        }
    });
    
    // Filter and render
    const nextTodos = todos.filter(t => t.time_horizon === 'next' && t.status !== 'completed');
    const soonTodos = todos.filter(t => t.time_horizon === 'soon' && t.status !== 'completed');
    const laterTodos = todos.filter(t => t.time_horizon === 'later' && t.status !== 'completed');
    
    renderTodoList(nextList, nextTodos, 'next', editModeElements);
    renderTodoList(soonList, soonTodos, 'soon', editModeElements);
    renderTodoList(laterList, laterTodos, 'later', editModeElements);
    
    // Setup drag and drop for all lists
    setupDragAndDrop();
}

/**
 * Render a list of todos
 */
function renderTodoList(container, todoList, timeHorizon, editModeElements) {
    if (!container) {
        return;
    }
    
    // Set data attribute for drop zone identification
    container.setAttribute('data-time-horizon', timeHorizon);
    
    todoList.forEach(todo => {
        // If this todo is in edit mode, re-use the existing element to preserve edit state
        if (editModeElements && editModeElements.has(String(todo.id))) {
            const existingEl = editModeElements.get(String(todo.id));
            container.appendChild(existingEl);
            return;
        }
        
        const todoEl = document.createElement('div');
        todoEl.className = 'todo-item';
        
        // Add status class for styling
        if (todo.status === 'processing') {
            todoEl.classList.add('status-processing');
        } else if (todo.status === 'completed') {
            todoEl.classList.add('status-completed');
        } else if (todo.status === 'processed') {
            todoEl.classList.add('status-processed');
        } else {
            todoEl.classList.add('status-pending');
        }
        
        todoEl.setAttribute('data-todo-id', todo.id);
        // Make todos draggable (except completed ones)
        if (todo.status !== 'completed') {
            todoEl.setAttribute('draggable', 'true');
            todoEl.classList.add('draggable-todo');
        }
        
        // Status indicator (clickable to reprocess)
        const statusDiv = document.createElement('div');
        statusDiv.className = 'todo-status';
        
        const statusBadge = document.createElement('span');
        statusBadge.className = `status-badge status-${todo.status}`;
        statusBadge.setAttribute('data-todo-id', todo.id);
        
        // Make status badge clickable (except for completed todos)
        // Allow reprocessing for pending, processed, and processing todos
        if (todo.status !== 'completed') {
            statusBadge.classList.add('status-badge-clickable');
            statusBadge.setAttribute('title', 'Click to reprocess this todo');
            statusBadge.addEventListener('click', (e) => {
                e.stopPropagation();
                handleReprocessTodo(todo.id, statusBadge);
            });
        }
        
        if (todo.status === 'processing') {
            statusBadge.textContent = 'Processing...';
            const spinner = document.createElement('span');
            spinner.className = 'spinner';
            statusBadge.appendChild(spinner);
        } else if (todo.status === 'pending') {
            statusBadge.textContent = 'Pending';
        } else if (todo.status === 'processed') {
            statusBadge.textContent = 'Processed';
        } else if (todo.status === 'completed') {
            statusBadge.textContent = 'Completed';
        }
        
        statusDiv.appendChild(statusBadge);
        
        // Text content
        const textSpan = document.createElement('span');
        textSpan.className = 'todo-text';
        textSpan.textContent = todo.text;
        
        // Due date display
        if (todo.due_date) {
            const dueDateDiv = document.createElement('div');
            dueDateDiv.className = 'todo-due-date';
            dueDateDiv.textContent = formatDate(todo.due_date);
            
            // Make due date clickable to edit
            dueDateDiv.classList.add('due-date-editable');
            dueDateDiv.setAttribute('title', 'Click to edit due date');
            dueDateDiv.addEventListener('click', (e) => {
                e.stopPropagation();
                handleEditDueDate(todo.id, dueDateDiv, todo.due_date);
            });
            
            todoEl.appendChild(dueDateDiv);
        }
        
        // Actions
        const actionsDiv = document.createElement('div');
        actionsDiv.className = 'todo-actions';
        
        const editBtn = document.createElement('button');
        editBtn.className = 'btn btn-small btn-secondary';
        editBtn.textContent = 'Edit';
        editBtn.disabled = todo.status === 'completed';
        editBtn.setAttribute('draggable', 'false');
        editBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            handleEditTodo(todo.id, todoEl, todo);
        });
        
        const completeBtn = document.createElement('button');
        completeBtn.className = 'btn btn-small btn-complete';
        completeBtn.textContent = 'Complete';
        completeBtn.disabled = todo.status === 'completed';
        completeBtn.setAttribute('draggable', 'false');
        completeBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            handleCompleteTodo(todo.id);
        });
        
        const deleteBtn = document.createElement('button');
        deleteBtn.className = 'btn btn-small btn-delete';
        deleteBtn.textContent = 'Delete';
        deleteBtn.setAttribute('draggable', 'false');
        deleteBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            handleDeleteTodo(todo.id);
        });
        
        actionsDiv.appendChild(editBtn);
        actionsDiv.appendChild(completeBtn);
        actionsDiv.appendChild(deleteBtn);
        
        // Metadata display (tags, priority, etc.)
        const metadataDiv = document.createElement('div');
        metadataDiv.className = 'todo-metadata';
        
        if (todo.metadata) {
            if (todo.metadata.category_tags && todo.metadata.category_tags.length > 0) {
                const tagsDiv = document.createElement('div');
                tagsDiv.className = 'todo-tags';
                todo.metadata.category_tags.forEach(tag => {
                    const tagSpan = document.createElement('span');
                    tagSpan.className = 'tag';
                    const source = todo.metadata.tag_sources && todo.metadata.tag_sources[tag];
                    if (source === 'ai') {
                        tagSpan.classList.add('tag-ai');
                    } else {
                        tagSpan.classList.add('tag-user');
                    }
                    tagSpan.textContent = tag;
                    tagsDiv.appendChild(tagSpan);
                });
                metadataDiv.appendChild(tagsDiv);
            }
            
            if (todo.metadata.priority) {
                const prioritySpan = document.createElement('span');
                prioritySpan.className = `priority priority-${todo.metadata.priority.toLowerCase()}`;
                prioritySpan.textContent = `Priority: ${todo.metadata.priority}`;
                metadataDiv.appendChild(prioritySpan);
            }
        }
        
        todoEl.appendChild(statusDiv);
        todoEl.appendChild(textSpan);
        if (metadataDiv.children.length > 0) {
            todoEl.appendChild(metadataDiv);
        }
        todoEl.appendChild(actionsDiv);
        container.appendChild(todoEl);
    });
}

/**
 * Setup drag and drop functionality
 */
function setupDragAndDrop() {
    const todoLists = document.querySelectorAll('.todo-list');
    const todoItems = document.querySelectorAll('.draggable-todo');
    
    // Setup drag events for todo items
    todoItems.forEach(item => {
        item.addEventListener('dragstart', handleDragStart);
        item.addEventListener('dragend', handleDragEnd);
    });
    
    // Setup drop events for todo lists
    todoLists.forEach(list => {
        list.addEventListener('dragover', handleDragOver);
        list.addEventListener('dragenter', handleDragEnter);
        list.addEventListener('dragleave', handleDragLeave);
        list.addEventListener('drop', handleDrop);
    });
}

/**
 * Handle drag start
 */
function handleDragStart(e) {
    const todoId = e.target.getAttribute('data-todo-id');
    e.dataTransfer.setData('text/plain', todoId);
    e.dataTransfer.effectAllowed = 'move';
    e.target.classList.add('dragging');
    
    // Add a semi-transparent clone as drag image
    const dragImage = e.target.cloneNode(true);
    dragImage.style.opacity = '0.5';
    document.body.appendChild(dragImage);
    dragImage.style.position = 'absolute';
    dragImage.style.top = '-1000px';
    e.dataTransfer.setDragImage(dragImage, e.offsetX, e.offsetY);
    setTimeout(() => document.body.removeChild(dragImage), 0);
}

/**
 * Handle drag end
 */
function handleDragEnd(e) {
    e.target.classList.remove('dragging');
    // Remove drop-zone highlighting from all lists
    document.querySelectorAll('.todo-list').forEach(list => {
        list.classList.remove('drag-over');
    });
}

/**
 * Handle drag over (required to allow drop)
 */
function handleDragOver(e) {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
}

/**
 * Handle drag enter
 */
function handleDragEnter(e) {
    e.preventDefault();
    const list = e.currentTarget;
    // Only highlight if it's not the list the item came from
    const draggedTodoId = e.dataTransfer.getData('text/plain');
    const draggedItem = document.querySelector(`[data-todo-id="${draggedTodoId}"]`);
    if (draggedItem && !list.contains(draggedItem)) {
        list.classList.add('drag-over');
    }
}

/**
 * Handle drag leave
 */
function handleDragLeave(e) {
    // Only remove highlight if we're actually leaving the list (not entering a child)
    const list = e.currentTarget;
    if (!list.contains(e.relatedTarget)) {
        list.classList.remove('drag-over');
    }
}

/**
 * Handle drop
 */
async function handleDrop(e) {
    e.preventDefault();
    const list = e.currentTarget;
    list.classList.remove('drag-over');
    
    const todoId = e.dataTransfer.getData('text/plain');
    const targetHorizon = list.getAttribute('data-time-horizon');
    
    if (!todoId || !targetHorizon) {
        return;
    }
    
    // Find the todo
    const todo = todos.find(t => t.id === todoId);
    if (!todo) {
        return;
    }
    
    // Don't update if already in the correct horizon
    if (todo.time_horizon === targetHorizon) {
        return;
    }
    
    // Update the todo's time_horizon
    try {
        await updateTodo(todoId, { time_horizon: targetHorizon });
        
        // Update local state
        todo.time_horizon = targetHorizon;
        
        // Re-render to show the change
        renderTodos();
    } catch (error) {
        logger.error('Failed to update todo time horizon:', error);
        // Don't show error messages for auth errors (we're redirecting)
        if (!error.isAuthError) {
            showError(error.message || 'Failed to move todo. Please try again.');
        }
    }
}

/**
 * Handle editing due date for a todo
 */
async function handleEditDueDate(id, element, currentDueDate) {
    const currentText = formatDate(currentDueDate);
    const input = document.createElement('input');
    input.type = 'text';
    input.className = 'due-date-input-edit';
    input.value = currentText;
    input.placeholder = 'e.g., tomorrow at 3pm, next Friday';
    
    // Replace element with input
    element.replaceWith(input);
    input.focus();
    input.select();
    
    const finishEdit = async () => {
        const newValue = input.value.trim();
        let newDueDate = null;
        
        if (newValue) {
            newDueDate = parseNaturalDate(newValue);
            if (!newDueDate) {
                showError(`Invalid due date format: "${newValue}". Try formats like "tomorrow at 3pm", "next Friday", or "2024-03-15T14:30:00Z"`);
                // Restore original display
                const newElement = document.createElement('div');
                newElement.className = 'todo-due-date due-date-editable';
                newElement.textContent = currentText;
                newElement.setAttribute('title', 'Click to edit due date');
                newElement.addEventListener('click', (e) => {
                    e.stopPropagation();
                    handleEditDueDate(id, newElement, currentDueDate);
                });
                input.replaceWith(newElement);
                return;
            }
        }
        
        try {
            await updateTodo(id, { due_date: newDueDate || '' });
            
            // Update local state
            const todo = todos.find(t => t.id === id);
            if (todo) {
                todo.due_date = newDueDate;
            }
            
            // Re-render to show updated date
            renderTodos();
        } catch (error) {
            logger.error('Failed to update due date:', error);
            // Restore original display
            const newElement = document.createElement('div');
            newElement.className = 'todo-due-date due-date-editable';
            newElement.textContent = currentText;
            newElement.setAttribute('title', 'Click to edit due date');
            newElement.addEventListener('click', (e) => {
                e.stopPropagation();
                handleEditDueDate(id, newElement, currentDueDate);
            });
            input.replaceWith(newElement);
            
            if (!error.isAuthError) {
                showError(error.message || 'Failed to update due date. Please try again.');
            }
        }
    };
    
    // Remove blur handler - use explicit Enter/Escape keys instead
    // This prevents cards from closing when clicking on buttons/fields
    input.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            finishEdit();
        } else if (e.key === 'Escape') {
            // Restore original display
            const newElement = document.createElement('div');
            newElement.className = 'todo-due-date due-date-editable';
            newElement.textContent = currentText;
            newElement.setAttribute('title', 'Click to edit due date');
            newElement.addEventListener('click', (e) => {
                e.stopPropagation();
                handleEditDueDate(id, newElement, currentDueDate);
            });
            input.replaceWith(newElement);
        }
    });
}

/**
 * Deep clone utility for safely copying objects and arrays.
 * This prevents mutations to the original data structure.
 * 
 * Uses the native structuredClone() when available (modern browsers),
 * which handles complex types like Date, Map, Set, and circular references.
 * Falls back to a custom implementation for older browsers.
 * 
 * Note: The custom implementation handles the current data structures used in this app:
 * - Arrays of primitive values (strings, numbers)
 * - Objects with primitive values
 * - Nested objects and arrays
 * 
 * For more complex structures (e.g., objects with Date, Map, Set, or class instances),
 * the native structuredClone is preferred when available.
 * 
 * @param {*} obj - The value to clone (can be any type)
 * @returns {*} A deep clone of the input value
 */
function deepClone(obj) {
    // Use native structuredClone if available (modern browsers)
    if (typeof structuredClone === 'function') {
        return structuredClone(obj);
    }
    
    // Fallback implementation for older browsers
    if (obj === null || typeof obj !== 'object') {
        return obj;
    }
    
    if (Array.isArray(obj)) {
        return obj.map(item => deepClone(item));
    }
    
    const cloned = {};
    for (const key in obj) {
        if (Object.prototype.hasOwnProperty.call(obj, key)) {
            cloned[key] = deepClone(obj[key]);
        }
    }
    return cloned;
}

/**
 * Handle editing a todo
 */
async function handleEditTodo(id, todoEl, todo) {
    // Create working copies using deep clones to prevent mutations to the original todo object
    // This ensures that if the user cancels, the original todo data remains unchanged
    // Deep cloning is used even for simple structures to future-proof against more complex
    // nested data (e.g., if tag_sources values become objects with additional metadata)
    const originalText = todo.text;
    const workingTags = todo.metadata?.category_tags ? deepClone(todo.metadata.category_tags) : [];
    const workingTagSources = todo.metadata?.tag_sources ? deepClone(todo.metadata.tag_sources) : {};
    const originalDueDate = todo.due_date;

    // Create edit mode UI
    todoEl.classList.add('todo-edit-mode');
    todoEl.innerHTML = '';

    // Edit text input
    const textInput = document.createElement('input');
    textInput.type = 'text';
    textInput.className = 'todo-edit-text';
    textInput.value = originalText;
    textInput.placeholder = 'Todo text...';
    todoEl.appendChild(textInput);

    // Tags editor
    const tagsContainer = document.createElement('div');
    tagsContainer.className = 'todo-tags-edit';
    
    const tagsLabel = document.createElement('label');
    tagsLabel.textContent = 'Tags:';
    tagsLabel.style.display = 'block';
    tagsLabel.style.marginBottom = '5px';
    tagsLabel.style.fontWeight = '500';
    tagsContainer.appendChild(tagsLabel);

    const tagsDiv = document.createElement('div');
    tagsDiv.style.display = 'flex';
    tagsDiv.style.flexWrap = 'wrap';
    tagsDiv.style.gap = '6px';
    tagsDiv.style.marginBottom = '10px';

    // Render existing tags
    workingTags.forEach(tag => {
        const tagChip = createTagChip(tag, workingTagSources[tag] === 'ai', () => {
            // Remove tag handler
            const index = workingTags.indexOf(tag);
            if (index > -1) {
                workingTags.splice(index, 1);
                delete workingTagSources[tag];
                renderTagsEditor(tagsDiv, workingTags, workingTagSources);
            }
        });
        tagsDiv.appendChild(tagChip);
    });

    // Add tag input
    const addTagInput = document.createElement('input');
    addTagInput.type = 'text';
    addTagInput.className = 'todo-tags-input';
    addTagInput.placeholder = 'Add tag...';
    addTagInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            const tagName = addTagInput.value.trim();
            if (tagName && !workingTags.includes(tagName)) {
                workingTags.push(tagName);
                workingTagSources[tagName] = 'user';
                addTagInput.value = '';
                renderTagsEditor(tagsDiv, workingTags, workingTagSources);
            }
        }
    });
    tagsDiv.appendChild(addTagInput);
    tagsContainer.appendChild(tagsDiv);

    // Due date editor
    const dueDateContainer = document.createElement('div');
    dueDateContainer.style.marginBottom = '10px';
    const dueDateLabel = document.createElement('label');
    dueDateLabel.textContent = 'Due Date:';
    dueDateLabel.style.display = 'block';
    dueDateLabel.style.marginBottom = '5px';
    dueDateLabel.style.fontWeight = '500';
    dueDateContainer.appendChild(dueDateLabel);
    
    const dueDateInput = document.createElement('input');
    dueDateInput.type = 'text';
    dueDateInput.className = 'todo-edit-text';
    dueDateInput.value = originalDueDate ? formatDate(originalDueDate) : '';
    dueDateInput.placeholder = 'e.g., tomorrow at 3pm, next Friday';
    dueDateContainer.appendChild(dueDateInput);

    // Action buttons
    const editActions = document.createElement('div');
    editActions.className = 'todo-edit-actions';
    
    const saveBtn = document.createElement('button');
    saveBtn.className = 'btn btn-small btn-primary todo-edit-button';
    saveBtn.textContent = 'Save';
    saveBtn.addEventListener('click', async () => {
        await saveTodoEdit(id, textInput.value, workingTags, dueDateInput.value, todoEl);
    });
    
    const cancelBtn = document.createElement('button');
    cancelBtn.className = 'btn btn-small btn-secondary todo-edit-button';
    cancelBtn.textContent = 'Cancel';
    cancelBtn.addEventListener('click', () => {
        // Remove edit mode class before reloading so renderTodos() will actually re-render
        todoEl.classList.remove('todo-edit-mode');
        // Re-render todo to exit edit mode
        loadTodos();
    });
    
    editActions.appendChild(saveBtn);
    editActions.appendChild(cancelBtn);

    todoEl.appendChild(tagsContainer);
    todoEl.appendChild(dueDateContainer);
    todoEl.appendChild(editActions);

    // Focus text input
    textInput.focus();
    textInput.select();

    // Helper function to render tags editor
    function renderTagsEditor(container, tags, tagSources) {
        container.innerHTML = '';
        tags.forEach(tag => {
            const tagChip = createTagChip(tag, tagSources[tag] === 'ai', () => {
                const index = tags.indexOf(tag);
                if (index > -1) {
                    tags.splice(index, 1);
                    delete tagSources[tag];
                    renderTagsEditor(container, tags, tagSources);
                }
            });
            container.appendChild(tagChip);
        });
        container.appendChild(addTagInput);
    }
}

/**
 * Create a tag chip element
 */
function createTagChip(tagName, isAI, onRemove) {
    const chip = document.createElement('span');
    chip.className = `tag-chip ${isAI ? 'tag-ai' : 'tag-user'}`;
    chip.textContent = tagName;
    
    // Allow removing all tags in edit mode (both AI and user tags)
    const removeBtn = document.createElement('span');
    removeBtn.className = 'tag-chip-remove';
    removeBtn.textContent = ' ×';
    removeBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        onRemove();
    });
    chip.appendChild(removeBtn);
    
    return chip;
}

/**
 * Save todo edits
 */
async function saveTodoEdit(id, text, tags, dueDateText, todoEl) {
    try {
        const updates = {};
        
        // Update text if changed
        if (text.trim()) {
            updates.text = text.trim();
        }
        
        // Update tags
        updates.tags = tags;
        
        // Update due date
        if (dueDateText.trim()) {
            const parsedDate = parseNaturalDate(dueDateText.trim());
            if (parsedDate) {
                // parseNaturalDate already returns an ISO string, so use it directly
                updates.due_date = parsedDate;
            } else {
                showError(`Invalid due date format: "${dueDateText}". Try formats like "tomorrow at 3pm", "next Friday", or "2024-03-15T14:30:00Z"`);
                return;
            }
        } else {
            updates.due_date = '';
        }
        
        // Update todo
        await updateTodo(id, updates);
        
        // Optimistically set status to processing before triggering reprocessing
        // This ensures the UI shows "processing" immediately
        const todo = todos.find(t => t.id === id);
        if (todo) {
            todo.status = 'processing';
            // Remove edit mode class and re-render to show processing status
            if (todoEl) {
                todoEl.classList.remove('todo-edit-mode');
            }
            renderTodos();
        }
        
        // Trigger reprocessing
        try {
            await analyzeTodo(id);
        } catch (error) {
            logger.error('Failed to trigger reprocessing:', error);
            // Don't fail the save if reprocessing fails
        }
        
        // Reload todos after a short delay to get the updated state from server
        // This gives the worker time to pick up the job
        setTimeout(async () => {
            await loadTodos();
        }, 500);
    } catch (error) {
        logger.error('Failed to save todo edit:', error);
        if (!error.isAuthError) {
            showError(error.message || 'Failed to save todo. Please try again.');
        }
    }
}


