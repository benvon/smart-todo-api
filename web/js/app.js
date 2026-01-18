// Main application logic

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
    
    // Setup logout button
    const logoutButton = document.getElementById('logout-button');
    if (logoutButton) {
        logoutButton.addEventListener('click', logout);
    }
    
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
    
    // Load todos
    await loadTodos();
});

/**
 * Load todos from API
 */
async function loadTodos() {
    try {
        const response = await getTodos();
        todos = response.data || [];
        renderTodos();
    } catch (error) {
        console.error('Failed to load todos:', error);
        showError('Failed to load todos. Please refresh the page.');
    }
}

/**
 * Handle adding a new todo
 */
async function handleAddTodo() {
    const input = document.getElementById('todo-input');
    const text = input.value.trim();
    
    if (!text) {
        return;
    }
    
    try {
        const response = await createTodo(text);
        todos.push(response.data);
        input.value = '';
        renderTodos();
    } catch (error) {
        console.error('Failed to create todo:', error);
        showError('Failed to create todo. Please try again.');
    }
}

/**
 * Handle completing a todo
 */
async function handleCompleteTodo(id) {
    try {
        const response = await completeTodo(id);
        const updatedTodo = response.data;
        
        // Update in local array
        const index = todos.findIndex(t => t.id === id);
        if (index !== -1) {
            todos[index] = updatedTodo;
        }
        
        renderTodos();
    } catch (error) {
        console.error('Failed to complete todo:', error);
        showError('Failed to complete todo. Please try again.');
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
        console.error('Failed to delete todo:', error);
        showError('Failed to delete todo. Please try again.');
    }
}

/**
 * Render todos in columns
 */
function renderTodos() {
    const nowList = document.getElementById('todos-now');
    const soonList = document.getElementById('todos-soon');
    const laterList = document.getElementById('todos-later');
    
    // Clear existing todos
    [nowList, soonList, laterList].forEach(list => {
        if (list) list.innerHTML = '';
    });
    
    // Filter and render
    const nowTodos = todos.filter(t => t.time_horizon === 'now' && t.status !== 'completed');
    const soonTodos = todos.filter(t => t.time_horizon === 'soon' && t.status !== 'completed');
    const laterTodos = todos.filter(t => t.time_horizon === 'later' && t.status !== 'completed');
    
    renderTodoList(nowList, nowTodos);
    renderTodoList(soonList, soonTodos);
    renderTodoList(laterList, laterTodos);
}

/**
 * Render a list of todos
 */
function renderTodoList(container, todoList) {
    if (!container) return;
    
    todoList.forEach(todo => {
        const todoEl = document.createElement('div');
        todoEl.className = 'todo-item';
        todoEl.innerHTML = `
            <span class="todo-text">${escapeHtml(todo.text)}</span>
            <div class="todo-actions">
                <button class="btn btn-small btn-complete" onclick="handleCompleteTodo('${todo.id}')">Complete</button>
                <button class="btn btn-small btn-delete" onclick="handleDeleteTodo('${todo.id}')">Delete</button>
            </div>
        `;
        container.appendChild(todoEl);
    });
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
 * Show error message
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
