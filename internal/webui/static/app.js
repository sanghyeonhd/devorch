// DevOrch Web UI Application
(function() {
    'use strict';

    // State
    const state = {
        sessionId: null,
        sessions: [],
        messages: [],
        isStreaming: false,
        eventSource: null,
        settings: {
            model: 'claude-sonnet-4-20250514',
            temperature: 0.7
        }
    };

    // DOM Elements
    const elements = {
        messages: document.getElementById('messages'),
        userInput: document.getElementById('user-input'),
        sendBtn: document.getElementById('send-btn'),
        sessionList: document.getElementById('session-list'),
        newSessionBtn: document.getElementById('new-session'),
        toggleSidebarBtn: document.getElementById('toggle-sidebar'),
        sidebar: document.getElementById('sidebar'),
        modelSelect: document.getElementById('model-select'),
        temperature: document.getElementById('temperature'),
        tempValue: document.getElementById('temp-value'),
        status: document.getElementById('status'),
        tokenCount: document.getElementById('token-count')
    };

    // Initialize
    function init() {
        bindEvents();
        connectSSE();
        loadSessions();
        updateStatus('Connected');
    }

    // Event bindings
    function bindEvents() {
        elements.sendBtn.addEventListener('click', sendMessage);
        
        elements.userInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                sendMessage();
            }
        });

        elements.newSessionBtn.addEventListener('click', createNewSession);
        
        elements.toggleSidebarBtn.addEventListener('click', () => {
            elements.sidebar.classList.toggle('hidden');
        });

        elements.modelSelect.addEventListener('change', (e) => {
            state.settings.model = e.target.value;
        });

        elements.temperature.addEventListener('input', (e) => {
            state.settings.temperature = parseFloat(e.target.value);
            elements.tempValue.textContent = e.target.value;
        });
    }

    // SSE Connection
    function connectSSE() {
        const clientId = 'web-' + Date.now();
        state.eventSource = new EventSource('/events?client_id=' + clientId);

        state.eventSource.addEventListener('connected', (e) => {
            console.log('SSE connected:', JSON.parse(e.data));
            updateStatus('Connected');
        });

        state.eventSource.addEventListener('message', (e) => {
            const data = JSON.parse(e.data);
            handleServerMessage(data);
        });

        state.eventSource.addEventListener('stream', (e) => {
            const data = JSON.parse(e.data);
            handleStreamChunk(data);
        });

        state.eventSource.addEventListener('error', (e) => {
            handleServerError(JSON.parse(e.data));
        });

        state.eventSource.onerror = () => {
            updateStatus('Disconnected - Reconnecting...');
            setTimeout(connectSSE, 3000);
        };
    }

    // API calls
    async function apiCall(endpoint, method = 'GET', body = null) {
        const options = {
            method,
            headers: {
                'Content-Type': 'application/json'
            }
        };

        if (body) {
            options.body = JSON.stringify(body);
        }

        const response = await fetch('/api' + endpoint, options);
        return response.json();
    }

    // Session management
    async function loadSessions() {
        try {
            const sessions = await apiCall('/sessions');
            state.sessions = sessions || [];
            renderSessionList();
        } catch (err) {
            console.error('Failed to load sessions:', err);
        }
    }

    async function createNewSession() {
        try {
            const session = await apiCall('/sessions', 'POST', {
                name: 'New Session ' + new Date().toLocaleString()
            });
            state.sessions.unshift(session);
            selectSession(session.id);
            renderSessionList();
        } catch (err) {
            console.error('Failed to create session:', err);
        }
    }

    function selectSession(sessionId) {
        state.sessionId = sessionId;
        state.messages = [];
        renderMessages();
        renderSessionList();
        loadSessionMessages(sessionId);
    }

    async function loadSessionMessages(sessionId) {
        try {
            const messages = await apiCall('/sessions/' + sessionId + '/messages');
            state.messages = messages || [];
            renderMessages();
        } catch (err) {
            console.error('Failed to load messages:', err);
        }
    }

    // Message handling
    async function sendMessage() {
        const text = elements.userInput.value.trim();
        if (!text || state.isStreaming) return;

        // Add user message
        addMessage('user', text);
        elements.userInput.value = '';

        // Send to server
        state.isStreaming = true;
        updateStatus('Thinking...');
        elements.sendBtn.disabled = true;

        try {
            await apiCall('/chat', 'POST', {
                session_id: state.sessionId,
                message: text,
                model: state.settings.model,
                temperature: state.settings.temperature
            });
        } catch (err) {
            console.error('Failed to send message:', err);
            addMessage('system', 'Error: ' + err.message);
            state.isStreaming = false;
            elements.sendBtn.disabled = false;
            updateStatus('Error');
        }
    }

    function handleServerMessage(data) {
        if (data.type === 'assistant') {
            addMessage('assistant', data.content);
            updateTokenCount(data.tokens);
        }
        state.isStreaming = false;
        elements.sendBtn.disabled = false;
        updateStatus('Ready');
    }

    function handleStreamChunk(data) {
        // Find or create assistant message
        let lastMsg = state.messages[state.messages.length - 1];
        if (!lastMsg || lastMsg.role !== 'assistant' || !lastMsg.streaming) {
            lastMsg = { role: 'assistant', content: '', streaming: true };
            state.messages.push(lastMsg);
        }

        lastMsg.content += data.content;
        renderMessages();

        if (data.done) {
            lastMsg.streaming = false;
            state.isStreaming = false;
            elements.sendBtn.disabled = false;
            updateStatus('Ready');
        }
    }

    function handleServerError(data) {
        addMessage('system', 'Error: ' + data.error);
        state.isStreaming = false;
        elements.sendBtn.disabled = false;
        updateStatus('Error');
    }

    function addMessage(role, content) {
        state.messages.push({ role, content });
        renderMessages();
    }

    // Rendering
    function renderMessages() {
        elements.messages.innerHTML = state.messages.map(msg => {
            const content = formatContent(msg.content);
            return `<div class="message ${msg.role}">${content}</div>`;
        }).join('');

        // Scroll to bottom
        elements.messages.scrollTop = elements.messages.scrollHeight;
    }

    function renderSessionList() {
        elements.sessionList.innerHTML = state.sessions.map(session => {
            const active = session.id === state.sessionId ? 'active' : '';
            return `<li class="${active}" onclick="window.selectSession('${session.id}')">${session.name}</li>`;
        }).join('');
    }

    function formatContent(content) {
        // Escape HTML
        content = content
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');

        // Code blocks
        content = content.replace(/```(\w*)\n([\s\S]*?)```/g, (_, lang, code) => {
            return `<pre><code class="language-${lang}">${code}</code></pre>`;
        });

        // Inline code
        content = content.replace(/`([^`]+)`/g, '<code>$1</code>');

        // Bold
        content = content.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');

        // Italic
        content = content.replace(/\*(.+?)\*/g, '<em>$1</em>');

        // Line breaks
        content = content.replace(/\n/g, '<br>');

        return content;
    }

    function updateStatus(status) {
        elements.status.textContent = status;
    }

    function updateTokenCount(tokens) {
        elements.tokenCount.textContent = 'Tokens: ' + tokens;
    }

    // Expose to window for onclick handlers
    window.selectSession = selectSession;

    // Start
    init();
})();
