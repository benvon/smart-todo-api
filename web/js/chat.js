// Chat functionality

import { sendChatMessage, getAIContext, updateAIContext } from './api.js';
import logger from './logger.js';

let currentContext = '';

/**
 * Initialize chat interface
 */
function initChat() {
    const chatInput = document.getElementById('chat-input');
    const chatSendButton = document.getElementById('chat-send-button');
    const chatMessages = document.getElementById('chat-messages');
    const loadContextButton = document.getElementById('chat-load-context-button');
    const saveContextButton = document.getElementById('chat-save-context-button');
    
    if (!chatInput || !chatSendButton || !chatMessages) {
        return;
    }
    
    // Load initial context
    loadContext();
    
    // Add welcome message
    addChatMessage('assistant', 'Hello! I\'m your AI assistant. I can help you manage your todos, understand your preferences, and answer questions about your tasks. How can I help you today?');
    
    // Handle send button click
    chatSendButton.addEventListener('click', handleSendMessage);
    
    // Handle Enter key in chat input
    chatInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            handleSendMessage();
        }
    });
    
    // Handle load context button
    if (loadContextButton) {
        loadContextButton.addEventListener('click', handleLoadContext);
    }
    
    // Handle save context button
    if (saveContextButton) {
        saveContextButton.addEventListener('click', handleSaveContext);
    }
}

/**
 * Load context from API
 */
async function loadContext() {
    try {
        const response = await getAIContext();
        if (response.data && response.data.context_summary) {
            currentContext = response.data.context_summary;
        }
    } catch (error) {
        logger.error('Failed to load context:', error);
    }
}

/**
 * Handle loading context into chat input
 */
async function handleLoadContext() {
    const chatInput = document.getElementById('chat-input');
    if (!chatInput) {
        return;
    }
    
    try {
        const response = await getAIContext();
        if (response.data && response.data.context_summary) {
            currentContext = response.data.context_summary;
            chatInput.value = currentContext;
            chatInput.focus();
        } else {
            chatInput.value = '';
        }
    } catch (error) {
        logger.error('Failed to load context:', error);
        if (!error.isAuthError) {
            showError(error.message || 'Failed to load context');
        }
    }
}

/**
 * Handle saving context from chat input
 */
async function handleSaveContext() {
    const chatInput = document.getElementById('chat-input');
    const saveButton = document.getElementById('chat-save-context-button');
    
    if (!chatInput || !saveButton) {
        return;
    }
    
    const contextText = chatInput.value.trim();
    
    // Disable button while saving
    saveButton.disabled = true;
    const originalText = saveButton.textContent;
    saveButton.textContent = 'Saving...';
    
    try {
        await updateAIContext(contextText);
        currentContext = contextText;
        saveButton.textContent = 'Saved!';
        setTimeout(() => {
            saveButton.textContent = originalText;
        }, 2000);
    } catch (error) {
        logger.error('Failed to save context:', error);
        if (!error.isAuthError) {
            showError(error.message || 'Failed to save context');
        }
        saveButton.textContent = originalText;
    } finally {
        saveButton.disabled = false;
    }
}

/**
 * Append AI response to context
 */
async function appendToContext(text) {
    const newContext = currentContext 
        ? `${currentContext}\n\n${text}`
        : text;
    
    await updateAIContext(newContext);
    currentContext = newContext;
}

/**
 * Handle sending a chat message
 */
async function handleSendMessage() {
    const chatInput = document.getElementById('chat-input');
    const chatSendButton = document.getElementById('chat-send-button');
    const message = chatInput.value.trim();
    
    if (!message) {
        return;
    }
    
    // Add user message to UI
    addChatMessage('user', message);
    
    // Clear input
    chatInput.value = '';
    
    // Disable input while processing
    chatInput.disabled = true;
    chatSendButton.disabled = true;
    chatSendButton.textContent = 'Sending...';
    
    try {
        // Send message to API
        const response = await sendChatMessage(message);
        
        // Response format: {success: true, data: {message: "...", summary: "...", needs_update: true}, timestamp: ...}
        if (response.data && response.data.message) {
            addChatMessage('assistant', response.data.message);
        } else {
            throw new Error('Invalid response from server');
        }
    } catch (error) {
        logger.error('Failed to send chat message:', error);
        // Don't show error messages for auth errors (we're redirecting)
        if (!error.isAuthError) {
            addChatMessage('assistant', `Sorry, I encountered an error: ${error.message}. Please try again.`);
        }
    } finally {
        // Re-enable input
        chatInput.disabled = false;
        chatSendButton.disabled = false;
        chatSendButton.textContent = 'Send';
        chatInput.focus();
    }
}

/**
 * Add a message to the chat UI
 */
function addChatMessage(role, content) {
    const chatMessages = document.getElementById('chat-messages');
    if (!chatMessages) {
        return;
    }
    
    const messageDiv = document.createElement('div');
    messageDiv.className = `chat-message chat-message-${role}`;
    
    const contentDiv = document.createElement('div');
    contentDiv.className = 'chat-message-content';
    contentDiv.textContent = content;
    
    messageDiv.appendChild(contentDiv);
    
    // Add append button for assistant messages
    if (role === 'assistant') {
        const appendBtn = document.createElement('button');
        appendBtn.className = 'btn btn-small btn-secondary';
        appendBtn.textContent = 'Append to Context';
        appendBtn.style.marginTop = '8px';
        appendBtn.style.fontSize = '11px';
        appendBtn.addEventListener('click', async () => {
            const originalText = appendBtn.textContent;
            try {
                await appendToContext(content);
                appendBtn.textContent = 'Appended!';
                appendBtn.disabled = true;
                setTimeout(() => {
                    appendBtn.textContent = originalText;
                    appendBtn.disabled = false;
                }, 2000);
            } catch (error) {
                logger.error('Failed to append to context:', error);
                appendBtn.textContent = 'Failed';
                appendBtn.classList.add('btn-danger');
                setTimeout(() => {
                    appendBtn.textContent = originalText;
                    appendBtn.classList.remove('btn-danger');
                }, 2000);
            }
        });
        messageDiv.appendChild(appendBtn);
    }
    
    chatMessages.appendChild(messageDiv);
    
    // Scroll to bottom
    chatMessages.scrollTop = chatMessages.scrollHeight;
}

/**
 * Show error message (reuse from app.js if available)
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

// Export functions for ES module use
export { initChat, handleSendMessage, addChatMessage, handleLoadContext, handleSaveContext, appendToContext };

// Expose functions globally for backward compatibility
if (typeof window !== 'undefined') {
    window.initChat = initChat;
    window.handleSendMessage = handleSendMessage;
    window.addChatMessage = addChatMessage;
    window.handleLoadContext = handleLoadContext;
    window.handleSaveContext = handleSaveContext;
    window.appendToContext = appendToContext;
}
