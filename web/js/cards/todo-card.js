// Todo card builder: given a todo and handlers, returns a DOM node for one todo item.
// Used by todo-list rendering; does not handle drag-drop or list-level logic.

import { formatDate } from '../dateutils.js';

/**
 * Build a single todo card DOM element.
 * @param {Object} todo - Todo object (id, text, status, due_date, metadata, time_horizon, ...)
 * @param {Object} handlers - Event handlers
 * @param { (id: string) => void } handlers.onComplete
 * @param { (id: string) => void } handlers.onDelete
 * @param { (id: string, element: HTMLElement, todo: Object) => void } handlers.onEdit
 * @param { (id: string, statusBadge: HTMLElement) => void } handlers.onReprocess
 * @param { (id: string, element: HTMLElement, currentDueDate: string) => void } handlers.onEditDueDate
 * @returns {HTMLElement}
 */
export function buildTodoCard(todo, handlers) {
    const todoEl = document.createElement('div');
    todoEl.className = 'todo-item';

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
    if (todo.status !== 'completed') {
        todoEl.setAttribute('draggable', 'true');
        todoEl.classList.add('draggable-todo');
    }

    const statusDiv = document.createElement('div');
    statusDiv.className = 'todo-status';

    const statusBadge = document.createElement('span');
    statusBadge.className = `status-badge status-${todo.status}`;
    statusBadge.setAttribute('data-todo-id', todo.id);

    if (todo.status !== 'completed') {
        statusBadge.classList.add('status-badge-clickable');
        statusBadge.setAttribute('title', 'Click to reprocess this todo');
        statusBadge.addEventListener('click', (e) => {
            e.stopPropagation();
            handlers.onReprocess(todo.id, statusBadge);
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

    const textSpan = document.createElement('span');
    textSpan.className = 'todo-text';
    textSpan.textContent = todo.text;

    if (todo.due_date) {
        const dueDateDiv = document.createElement('div');
        dueDateDiv.className = 'todo-due-date due-date-editable';
        dueDateDiv.textContent = formatDate(todo.due_date);
        dueDateDiv.setAttribute('title', 'Click to edit due date');
        dueDateDiv.addEventListener('click', (e) => {
            e.stopPropagation();
            handlers.onEditDueDate(todo.id, dueDateDiv, todo.due_date);
        });
        todoEl.appendChild(dueDateDiv);
    }

    const actionsDiv = document.createElement('div');
    actionsDiv.className = 'todo-actions';

    const editBtn = document.createElement('button');
    editBtn.className = 'btn btn-small btn-secondary';
    editBtn.textContent = 'Edit';
    editBtn.disabled = todo.status === 'completed';
    editBtn.setAttribute('draggable', 'false');
    editBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        handlers.onEdit(todo.id, todoEl, todo);
    });

    const completeBtn = document.createElement('button');
    completeBtn.className = 'btn btn-small btn-complete';
    completeBtn.textContent = 'Complete';
    completeBtn.disabled = todo.status === 'completed';
    completeBtn.setAttribute('draggable', 'false');
    completeBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        handlers.onComplete(todo.id);
    });

    const deleteBtn = document.createElement('button');
    deleteBtn.className = 'btn btn-small btn-delete';
    deleteBtn.textContent = 'Delete';
    deleteBtn.setAttribute('draggable', 'false');
    deleteBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        handlers.onDelete(todo.id);
    });

    actionsDiv.appendChild(editBtn);
    actionsDiv.appendChild(completeBtn);
    actionsDiv.appendChild(deleteBtn);

    const metadataDiv = document.createElement('div');
    metadataDiv.className = 'todo-metadata';

    if (todo.metadata) {
        if (todo.metadata.category_tags && todo.metadata.category_tags.length > 0) {
            const tagsDiv = document.createElement('div');
            tagsDiv.className = 'todo-tags';
            todo.metadata.category_tags.forEach((tag) => {
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

    return todoEl;
}
