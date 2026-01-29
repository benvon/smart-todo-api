// Todo list: rendering, drag-drop, and todo actions. Uses todo-card builder.

import { getTodos, updateTodo, deleteTodo, completeTodo, analyzeTodo } from './api.js';
import { buildTodoCard } from './cards/todo-card.js';
import { formatDate, parseNaturalDate } from './dateutils.js';
import logger from './logger.js';
import { showError } from './error-utils.js';

let todos = [];

const handlers = {
    onComplete: (id) => handleCompleteTodo(id),
    onDelete: (id) => handleDeleteTodo(id),
    onEdit: (id, todoEl, todo) => handleEditTodo(id, todoEl, todo),
    onReprocess: (id, statusBadge) => handleReprocessTodo(id, statusBadge),
    onEditDueDate: (id, element, currentDueDate) => handleEditDueDate(id, element, currentDueDate)
};

/**
 * Load todos from API and re-render.
 */
export async function loadTodos() {
    try {
        const response = await getTodos();
        if (response.data && Array.isArray(response.data.todos)) {
            todos = response.data.todos;
        } else if (Array.isArray(response.data)) {
            todos = response.data;
        } else {
            todos = [];
        }
        renderTodos();
    } catch (error) {
        logger.error('Failed to load todos:', error);
        if (error.isAuthError) {
            return;
        }
        if (error.retryAfter) {
            showError(`Too many requests. Please wait ${error.retryAfter} seconds before trying again.`);
        } else {
            showError(error.message || 'Failed to load todos. Please refresh the page.');
        }
    }
}

function renderTodos() {
    const nextList = document.getElementById('todos-next');
    const soonList = document.getElementById('todos-soon');
    const laterList = document.getElementById('todos-later');

    const editModeTodos = document.querySelectorAll('.todo-item.todo-edit-mode');
    const editModeElements = new Map();
    editModeTodos.forEach(el => {
        const todoId = el.getAttribute('data-todo-id');
        if (todoId) {
            editModeElements.set(String(todoId), el);
        }
    });

    [nextList, soonList, laterList].forEach(list => {
        if (list) {
            list.innerHTML = '';
        }
    });

    const nextTodos = todos.filter(t => t.time_horizon === 'next' && t.status !== 'completed');
    const soonTodos = todos.filter(t => t.time_horizon === 'soon' && t.status !== 'completed');
    const laterTodos = todos.filter(t => t.time_horizon === 'later' && t.status !== 'completed');

    renderTodoList(nextList, nextTodos, 'next', editModeElements);
    renderTodoList(soonList, soonTodos, 'soon', editModeElements);
    renderTodoList(laterList, laterTodos, 'later', editModeElements);

    setupDragAndDrop();
}

function renderTodoList(container, todoList, timeHorizon, editModeElements) {
    if (!container) {
        return;
    }
    container.setAttribute('data-time-horizon', timeHorizon);

    todoList.forEach(todo => {
        if (editModeElements && editModeElements.has(String(todo.id))) {
            const existingEl = editModeElements.get(String(todo.id));
            container.appendChild(existingEl);
            return;
        }
        const todoEl = buildTodoCard(todo, handlers);
        container.appendChild(todoEl);
    });
}

function setupDragAndDrop() {
    const todoLists = document.querySelectorAll('.todo-list');
    const todoColumns = document.querySelectorAll('.todo-column');
    const todoItems = document.querySelectorAll('.draggable-todo');
    todoItems.forEach(item => {
        item.addEventListener('dragstart', handleDragStart);
        item.addEventListener('dragend', handleDragEnd);
    });
    todoLists.forEach(list => {
        list.addEventListener('dragover', handleDragOver);
        list.addEventListener('dragenter', handleDragEnter);
        list.addEventListener('dragleave', handleDragLeave);
        list.addEventListener('drop', handleDrop);
    });
    todoColumns.forEach(column => {
        column.addEventListener('dragover', handleDragOver);
        column.addEventListener('dragenter', handleDragEnter);
        column.addEventListener('dragleave', handleDragLeave);
        column.addEventListener('drop', handleDrop);
    });
}

function getListFromDropTarget(element) {
    if (element.classList.contains('todo-list')) {
        return element;
    }
    if (element.classList.contains('todo-column')) {
        return element.querySelector('.todo-list');
    }
    return null;
}

function handleDragStart(e) {
    const todoId = e.target.getAttribute('data-todo-id');
    e.dataTransfer.setData('text/plain', todoId);
    e.dataTransfer.effectAllowed = 'move';
    e.target.classList.add('dragging');
    const dragImage = e.target.cloneNode(true);
    dragImage.style.opacity = '0.5';
    document.body.appendChild(dragImage);
    dragImage.style.position = 'absolute';
    dragImage.style.top = '-1000px';
    e.dataTransfer.setDragImage(dragImage, e.offsetX, e.offsetY);
    setTimeout(() => document.body.removeChild(dragImage), 0);
}

function handleDragEnd(e) {
    e.target.classList.remove('dragging');
    document.querySelectorAll('.todo-list').forEach(list => {
        list.classList.remove('drag-over');
    });
}

function handleDragOver(e) {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
}

function handleDragEnter(e) {
    e.preventDefault();
    const list = getListFromDropTarget(e.currentTarget);
    if (!list) {
        return;
    }
    const draggedTodoId = e.dataTransfer.getData('text/plain');
    const draggedItem = document.querySelector(`[data-todo-id="${draggedTodoId}"]`);
    if (draggedItem && !list.contains(draggedItem)) {
        list.classList.add('drag-over');
    }
}

function handleDragLeave(e) {
    const list = getListFromDropTarget(e.currentTarget);
    if (!list) {
        return;
    }
    const related = e.relatedTarget;
    const column = list.closest('.todo-column');
    if (!related) {
        list.classList.remove('drag-over');
        return;
    }
    if (list.contains(related)) {
        return;
    }
    if (column && column.contains(related)) {
        return;
    }
    list.classList.remove('drag-over');
}

async function handleDrop(e) {
    e.preventDefault();
    const list = getListFromDropTarget(e.currentTarget);
    if (!list) {
        return;
    }
    list.classList.remove('drag-over');
    const todoId = e.dataTransfer.getData('text/plain');
    const targetHorizon = list.getAttribute('data-time-horizon');
    if (!todoId || !targetHorizon) {
        return;
    }
    const todo = todos.find(t => t.id === todoId);
    if (!todo || todo.time_horizon === targetHorizon) {
        return;
    }
    try {
        await updateTodo(todoId, { time_horizon: targetHorizon });
        todo.time_horizon = targetHorizon;
        renderTodos();
    } catch (error) {
        logger.error('Failed to update todo time horizon:', error);
        if (!error.isAuthError) {
            showError(error.message || 'Failed to move todo. Please try again.');
        }
    }
}

async function handleCompleteTodo(id) {
    try {
        const response = await completeTodo(id);
        const updatedTodo = response.data;
        if (updatedTodo) {
            const index = todos.findIndex(t => t.id === id);
            if (index !== -1) {
                todos[index] = updatedTodo;
            }
        }
        renderTodos();
    } catch (error) {
        logger.error('Failed to complete todo:', error);
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

async function handleDeleteTodo(id) {
    if (!confirm('Are you sure you want to delete this todo?')) {
        return;
    }
    try {
        await deleteTodo(id);
        todos = todos.filter(t => t.id !== id);
        renderTodos();
    } catch (error) {
        logger.error('Failed to delete todo:', error);
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

async function handleReprocessTodo(id, statusBadge) {
    if (statusBadge.dataset.processing === 'true') {
        showError('Todo is already being processed. Please wait.');
        return;
    }
    const todo = todos.find(t => t.id === id);
    if (todo && todo.status === 'processing') {
        showError('Todo is already being processed. Please wait.');
        return;
    }
    statusBadge.dataset.processing = 'true';
    const originalText = statusBadge.textContent.trim();
    const originalClass = statusBadge.className;
    statusBadge.textContent = 'Processing...';
    statusBadge.className = 'status-badge status-processing status-badge-clickable';
    statusBadge.setAttribute('title', 'Processing...');
    const existingSpinner = statusBadge.querySelector('.spinner');
    if (existingSpinner) {
        existingSpinner.remove();
    }
    const spinner = document.createElement('span');
    spinner.className = 'spinner';
    statusBadge.appendChild(spinner);

    try {
        await analyzeTodo(id);
        showError('Reprocessing started. The todo will be updated shortly.');
        setTimeout(() => loadTodos(), 1000);
    } catch (error) {
        logger.error('Failed to reprocess todo:', error);
        statusBadge.textContent = originalText;
        statusBadge.className = originalClass;
        statusBadge.setAttribute('title', 'Click to reprocess this todo');
        statusBadge.dataset.processing = 'false';
        const spinnerToRemove = statusBadge.querySelector('.spinner');
        if (spinnerToRemove) {
            spinnerToRemove.remove();
        }
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

async function handleEditDueDate(id, element, currentDueDate) {
    const currentText = formatDate(currentDueDate);
    const input = document.createElement('input');
    input.type = 'text';
    input.className = 'due-date-input-edit';
    input.value = currentText;
    input.placeholder = 'e.g., tomorrow at 3pm, next Friday';
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
            const todo = todos.find(t => t.id === id);
            if (todo) {
                todo.due_date = newDueDate;
            }
            renderTodos();
        } catch (error) {
            logger.error('Failed to update due date:', error);
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

    input.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            finishEdit();
        } else if (e.key === 'Escape') {
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

function deepClone(obj) {
    if (typeof structuredClone === 'function') {
        return structuredClone(obj);
    }
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

function createTagChip(tagName, isAI, onRemove) {
    const chip = document.createElement('span');
    chip.className = `tag-chip ${isAI ? 'tag-ai' : 'tag-user'}`;
    chip.textContent = tagName;
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

async function handleEditTodo(id, todoEl, todo) {
    const originalText = todo.text;
    const workingTags = todo.metadata?.category_tags ? deepClone(todo.metadata.category_tags) : [];
    const workingTagSources = todo.metadata?.tag_sources ? deepClone(todo.metadata.tag_sources) : {};
    const originalDueDate = todo.due_date;

    todoEl.classList.add('todo-edit-mode');
    todoEl.innerHTML = '';

    const textInput = document.createElement('input');
    textInput.type = 'text';
    textInput.className = 'todo-edit-text';
    textInput.value = originalText;
    textInput.placeholder = 'Todo text...';
    todoEl.appendChild(textInput);

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
    const chipsContainer = document.createElement('div');
    chipsContainer.style.display = 'flex';
    chipsContainer.style.flexWrap = 'wrap';
    chipsContainer.style.gap = '6px';
    tagsDiv.appendChild(chipsContainer);
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
                renderTagsEditor(chipsContainer, workingTags, workingTagSources);
            }
        }
    });
    tagsDiv.appendChild(addTagInput);
    tagsContainer.appendChild(tagsDiv);
    renderTagsEditor(chipsContainer, workingTags, workingTagSources);

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
        todoEl.classList.remove('todo-edit-mode');
        loadTodos();
    });
    editActions.appendChild(saveBtn);
    editActions.appendChild(cancelBtn);

    todoEl.appendChild(tagsContainer);
    todoEl.appendChild(dueDateContainer);
    todoEl.appendChild(editActions);

    textInput.focus();
    textInput.select();

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
    }
}

async function saveTodoEdit(id, text, tags, dueDateText, todoEl) {
    try {
        const updates = {};
        if (text.trim()) {
            updates.text = text.trim();
        }
        updates.tags = tags;
        if (dueDateText.trim()) {
            const parsedDate = parseNaturalDate(dueDateText.trim());
            if (parsedDate) {
                updates.due_date = parsedDate;
            } else {
                showError(`Invalid due date format: "${dueDateText}". Try formats like "tomorrow at 3pm", "next Friday", or "2024-03-15T14:30:00Z"`);
                return;
            }
        } else {
            updates.due_date = '';
        }

        await updateTodo(id, updates);
        const todo = todos.find(t => t.id === id);
        let originalStatus = null;
        if (todo) {
            originalStatus = todo.status;
            todo.status = 'processing';
            if (todoEl) {
                todoEl.classList.remove('todo-edit-mode');
            }
            renderTodos();
        }

        try {
            await analyzeTodo(id);
        } catch (error) {
            if (!error.isAuthError) {
                logger.error('Failed to trigger reprocessing:', error);
                if (todo) {
                    todo.status = originalStatus;
                    renderTodos();
                }
            }
        }
        setTimeout(() => loadTodos(), 500);
    } catch (error) {
        logger.error('Failed to save todo edit:', error);
        if (!error.isAuthError) {
            showError(error.message || 'Failed to save todo. Please try again.');
        }
    }
}
