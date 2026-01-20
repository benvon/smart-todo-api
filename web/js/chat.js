// Chat functionality

/**
 * Initialize chat interface
 */
function initChat() {
    const chatInput = document.getElementById('chat-input');
    const chatSendButton = document.getElementById('chat-send-button');
    const chatMessages = document.getElementById('chat-messages');
    
    if (!chatInput || !chatSendButton || !chatMessages) {
        return;
    }
    
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
        console.error('Failed to send chat message:', error);
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
    chatMessages.appendChild(messageDiv);
    
    // Scroll to bottom
    chatMessages.scrollTop = chatMessages.scrollHeight;
}
