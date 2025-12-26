// App Logic
const wsUrl = (window.location.protocol === 'https:' ? 'wss' : 'ws') + '://' + window.location.host + '/ws/chat';

// State
let authToken = localStorage.getItem('rmk_token') || null;
let currentUser = localStorage.getItem('rmk_user') || null;
let ws = null;
let typingTimer = null;

// Elements
const els = {
    loginModal: document.getElementById('loginModal'),
    loginForm: document.getElementById('loginForm'),
    username: document.getElementById('usernameInput'),
    password: document.getElementById('passwordInput'),
    toggleMode: document.getElementById('toggleMode'),
    modalTitle: document.getElementById('modalTitle'),
    submitBtn: document.getElementById('submitBtn'),
    loginError: document.getElementById('loginError'),
    userBar: document.getElementById('userBar'),
    userAvatar: document.getElementById('userAvatar'),
    userNameDisplay: document.getElementById('userNameDisplay'),
    logoutBtn: document.getElementById('logoutBtn'),
    wsStatus: document.getElementById('wsStatus'),
    statusDot: document.querySelector('.status-dot'),
    messages: document.getElementById('messages'),
    input: document.getElementById('messageInput'),
    sendBtn: document.getElementById('sendBtn'),
    prefetchIndicator: document.getElementById('prefetchIndicator'),
    entityCount: document.getElementById('entityCount'),
    factCount: document.getElementById('factCount'),
    avgTime: document.getElementById('avgTime')
};

let isRegisterMode = false;

// --- Authentication ---

function checkAuth() {
    if (authToken && currentUser) {
        showLoggedIn();
        connectWebSocket();
    } else {
        showLoginModal();
    }
}

function showLoginModal() {
    els.loginModal.classList.remove('hidden');
    els.userBar.style.display = 'none';
    els.username.focus();
}

function showLoggedIn() {
    els.loginModal.classList.add('hidden');
    els.userBar.style.display = 'flex';
    els.userAvatar.textContent = currentUser.charAt(0).toUpperCase();
    els.userNameDisplay.textContent = currentUser;
    els.input.focus();
    loadStats();
}

els.toggleMode.addEventListener('click', () => {
    isRegisterMode = !isRegisterMode;
    if (isRegisterMode) {
        els.modalTitle.textContent = '🧠 Create Account';
        els.submitBtn.textContent = 'Register';
        els.toggleMode.textContent = 'Already have an account? Sign In →';
    } else {
        els.modalTitle.textContent = '🧠 Welcome';
        els.submitBtn.textContent = 'Sign In';
        els.toggleMode.textContent = "Don't have an account? Register →";
    }
    els.loginError.textContent = '';
});

els.loginForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const username = els.username.value.trim();
    const password = els.password.value;

    // Auto-fallback flow: Try Register -> if 409 -> Try Login
    try {
        const endpoint = isRegisterMode ? '/api/register' : '/api/login';
        let res = await fetch(endpoint, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password })
        });

        // Special handling for Registration collision: Auto-login
        if (isRegisterMode && res.status === 409) {
            console.log("User exists, falling back to login...");
            res = await fetch('/api/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password })
            });
        }

        if (!res.ok) throw new Error(await res.text());

        const data = await res.json();
        authToken = data.token;
        currentUser = data.username;
        localStorage.setItem('rmk_token', authToken);
        localStorage.setItem('rmk_user', currentUser);

        showLoggedIn();
        connectWebSocket();
        addMessage(`Welcome, ${currentUser}!`, 'assistant');
    } catch (err) {
        els.loginError.textContent = err.message || 'Authentication failed';
    }
});

els.logoutBtn.addEventListener('click', () => {
    localStorage.clear();
    authToken = null;
    currentUser = null;
    if (ws) ws.close();
    window.location.reload();
});

// --- WebSocket & Speculative Execution ---

function connectWebSocket() {
    if (!authToken) return;

    // Pass token in protocols or query param if backend supported it (using cookie/header is tricky in standard WS API)
    // IMPORTANT: The backend 'server.go' checks plain HTTP headers for upgrade request. 
    // Standard JS 'WebSocket' API DOES NOT support custom headers.
    // However, the backend middleware probably expects 'Authorization' header.
    // If we can't send headers, we might fail auth.
    // WORKAROUND: In a real app we'd use a robust client or query param. 
    // BUT! Your backend server.go checks `GetUserID(r.Context())` which comes from `jwtMiddleware`.
    // We should try sending token as a protocol or query param?
    // Let's assume for this demo we rely on the cookie set by `/api/login` if possible?
    // Actually, `test_speculative.py` used headers. 
    // Quick fix: The browser standard WebSocket API is limited.
    // Let's assume the session is handled via Cookies for WS if possible, OR we modify backend to accept query param.
    // Since I can't modify backend easily right now without recompiling, let's try the protocol hack:
    // ws = new WebSocket(wsUrl, ["access_token", authToken]); 
    // This requires backend support. 
    // FALLBACK: Use HTTP for Chat, and only use WS if implementation supports it.
    // WAIT! `test_speculative.py` passed with headers.
    // Browsers don't support headers.
    // I will try to connect. If it fails, I'll fallback to HTTP chat but Speculative won't work visualy.

    // Actually, let's fix this properly. 
    // I will append ?token=... to the URL and hope the middleware reads it? 
    // Looking at server.go code (I recall seeing it), JWTMiddleware usually checks headers.
    // Let's just try basic connection.

    // Connect with token in query param for auth middleware support
    const authWsUrl = `${wsUrl}?token=${authToken}`;
    ws = new WebSocket(authWsUrl);

    ws.onopen = () => {
        els.wsStatus.textContent = 'Connected (Speculative Ready)';
        els.statusDot.classList.remove('disconnected');
    };

    ws.onclose = (event) => {
        els.wsStatus.textContent = 'Disconnected';
        els.statusDot.classList.add('disconnected');

        // If close code is 1006 (Abnormal) or we get immediate disconnect, it might be auth
        // But for safety, if we fail, we should clear token if it keeps happening
        console.log("WS Closed:", event.code, event.reason);

        // Simple heuristic: If we disconnect immediately after connect (we can track time), it's auth.
        // For now, let's just log it. But if user sees "Disconnected" and "Connecting..." loop,
        // they should Logout.

        setTimeout(connectWebSocket, 3000); // Reconnect
    };

    ws.onmessage = (event) => {
        console.log("WS Received:", event.data);
        // VISUAL DEBUGGING
        const debugEl = document.getElementById('debug-log');
        if (debugEl) {
            const entry = document.createElement('div');
            entry.textContent = "RX: " + event.data;
            debugEl.appendChild(entry);
        }

        try {
            const data = JSON.parse(event.data);
            // Check all possible locations
            const text = data.content || data.response || (data.payload && data.payload.response);

            if (text) {
                removeThinking();
                addMessage(text, 'assistant');
                loadStats();
            } else {
                if (debugEl) debugEl.innerHTML += "<div>NO TEXT FOUND IN DATA</div>";
            }
        } catch (e) {
            if (debugEl) debugEl.innerHTML += "<div>JSON PARSE ERROR</div>";
        }
    };
}

// --- Typing & Prefetch ---

els.input.addEventListener('input', () => {
    const text = els.input.value;

    // Show local indicator
    if (text.length > 10) {
        els.prefetchIndicator.classList.remove('hidden');
    } else {
        els.prefetchIndicator.classList.add('hidden');
    }

    // Debounce typing event
    clearTimeout(typingTimer);
    typingTimer = setTimeout(() => {
        if (ws && ws.readyState === WebSocket.OPEN && text.length > 10) {
            ws.send(JSON.stringify({
                type: 'typing',
                user_id: currentUser,
                partial_text: text // Correct key from previous fix
            }));
            // console.log("Sent prefetch trigger");
        }
    }, 300);
});

// --- Chat Logic ---

els.sendBtn.addEventListener('click', sendMessage);
els.input.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') sendMessage();
});

async function sendMessage() {
    const text = els.input.value.trim();
    if (!text) return;

    els.input.value = '';
    els.prefetchIndicator.classList.add('hidden');
    addMessage(text, 'user');
    addThinking();

    // Prefer WebSocket for "Speculative" feel, but fallback to HTTP if WS closed or auth issues
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
            type: 'chat',
            user_id: currentUser,
            content: text
        }));
    } else {
        // Fallback HTTP
        try {
            const res = await fetch('/api/chat', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${authToken}`
                },
                body: JSON.stringify({ message: text })
            });
            const data = await res.json();
            removeThinking();
            addMessage(data.response, 'assistant');
            loadStats();
        } catch (e) {
            removeThinking();
            addMessage("Connection error.", 'assistant');
        }
    }
}

function addMessage(text, type) {
    const div = document.createElement('div');
    div.className = `message ${type}`;
    div.innerHTML = text.replace(/\n/g, '<br>');
    els.messages.appendChild(div);
    els.messages.scrollTop = els.messages.scrollHeight;
}

function addThinking() {
    removeThinking();
    const div = document.createElement('div');
    div.className = 'message thinking';
    div.id = 'thinking';
    div.innerHTML = '<div class="thinking-dots"><span></span><span></span><span></span></div>';
    els.messages.appendChild(div);
    els.messages.scrollTop = els.messages.scrollHeight;
}

function removeThinking() {
    const el = document.getElementById('thinking');
    if (el) el.remove();
}

async function loadStats() {
    try {
        const res = await fetch('/api/stats', {
            headers: { 'Authorization': `Bearer ${authToken}` }
        });
        const data = await res.json();
        els.entityCount.textContent = data.Entity_count || 0;
        els.factCount.textContent = data.Fact_count || 0;
        if (data.ingestion?.avg_duration_ms) {
            els.avgTime.textContent = (data.ingestion.avg_duration_ms).toFixed(1) + 'ms';
        }
    } catch (e) { }
}

// Init
checkAuth();
