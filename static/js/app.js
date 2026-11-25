// PulseMatrix AI - Clean Enterprise SaaS Controller

document.addEventListener('DOMContentLoaded', () => {
    initCharts();
    initModalUX();
    initAutoDismissToasts();
});

let currentAIParsed = null;

function escapeHtml(str) {
    if (str === null || str === undefined) return '';
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

function initModalUX() {
    document.querySelectorAll('.modal-backdrop').forEach((backdrop) => {
        backdrop.addEventListener('click', (e) => {
            if (e.target === backdrop) {
                backdrop.style.display = 'none';
            }
        });
    });

    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            document.querySelectorAll('.modal-backdrop').forEach((m) => {
                m.style.display = 'none';
            });
            const sidebar = document.querySelector('.sidebar');
            if (sidebar) sidebar.classList.remove('open');
        }
    });
}

function initAutoDismissToasts() {
    const toast = document.getElementById('flash-alert');
    if (toast) {
        setTimeout(() => {
            if (toast.parentNode) toast.remove();
        }, 5000);
    }
}

window.toggleMobileSidebar = function() {
    const sidebar = document.querySelector('.sidebar');
    if (sidebar) sidebar.classList.toggle('open');
};

// Modal Controls - Bound to window object for inline onclick handlers
window.openReceiptModal = function() {
    const modal = document.getElementById('aiReceiptModal');
    if (modal) modal.style.display = 'flex';
};

window.closeReceiptModal = function() {
    const modal = document.getElementById('aiReceiptModal');
    if (modal) modal.style.display = 'none';
    const preview = document.getElementById('aiParsePreview');
    const btnConfirm = document.getElementById('btnConfirmAI');
    const btnRun = document.getElementById('btnRunAI');
    if (preview) preview.classList.add('hidden');
    if (btnConfirm) btnConfirm.classList.add('hidden');
    if (btnRun) btnRun.classList.remove('hidden');
};

window.openAddTxModal = function() {
    const modal = document.getElementById('addTxModal');
    if (modal) modal.style.display = 'flex';
};

window.closeAddTxModal = function() {
    const modal = document.getElementById('addTxModal');
    if (modal) modal.style.display = 'none';
};

window.toggleSettingsModal = function() {
    const modal = document.getElementById('settingsModal');
    if (modal) {
        modal.style.display = (modal.style.display === 'flex') ? 'none' : 'flex';
    }
};

// AI Autonomous Receipt Parser AJAX Handler
window.runAIReceiptParser = async function() {
    const rawText = document.getElementById('aiRawText').value.trim();
    if (!rawText) {
        alert('Please enter or paste receipt text.');
        return;
    }

    const btnRun = document.getElementById('btnRunAI');
    const previewBox = document.getElementById('aiParsePreview');

    btnRun.textContent = 'Processing...';
    btnRun.disabled = true;

    try {
        const resp = await fetch('/api/ai/parse-receipt', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ text: rawText })
        });

        if (!resp.ok) {
            throw new Error(`Parser Error HTTP ${resp.status}`);
        }

        const data = await resp.json();
        currentAIParsed = data;

        const confPct = ((Number(data.confidenceScore) || 0.85) * 100).toFixed(0);
        const amtNum = Number(data.amount) || 0;

        previewBox.classList.remove('hidden');
        previewBox.innerHTML = `
            <div style="font-weight:700; color:#10b981; margin-bottom:0.4rem;">Parsed Receipt Data (Confidence: ${escapeHtml(confPct)}%)</div>
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:0.5rem; font-size:0.86rem;">
                <div><strong>Merchant:</strong> ${escapeHtml(data.vendor)}</div>
                <div><strong>Amount:</strong> <span style="color:#6366f1; font-weight:700;">${escapeHtml(amtNum.toFixed(2))} ${escapeHtml(data.currency)}</span></div>
                <div><strong>Category:</strong> ${escapeHtml(data.suggestedCategory)}</div>
                <div><strong>Date:</strong> ${escapeHtml(data.date)}</div>
            </div>
            ${data.isAnomaly ? `<div class="badge badge-warning" style="margin-top:0.5rem;">⚠️ Anomaly: ${escapeHtml(data.anomalyReason)}</div>` : ''}
        `;

        document.getElementById('btnConfirmAI').classList.remove('hidden');
        btnRun.classList.add('hidden');
    } catch (err) {
        alert('Parsing failed: ' + err.message);
    } finally {
        btnRun.textContent = 'Parse Text';
        btnRun.disabled = false;
    }
};

// Confirm AI Parsed Entry & Post to Backend
window.confirmAITransaction = async function() {
    if (!currentAIParsed) return;

    const formData = new URLSearchParams();
    formData.append('vendor', currentAIParsed.vendor || 'Merchant');
    formData.append('amount', currentAIParsed.amount || 0);
    formData.append('currency', currentAIParsed.currency || 'USD');
    formData.append('date', currentAIParsed.date || '');
    formData.append('category_name', currentAIParsed.suggestedCategory || 'Miscellaneous');
    formData.append('payment_method', currentAIParsed.paymentMethod || 'Credit Card');
    formData.append('source', 'ai_ocr');
    formData.append('description', 'AI Receipt Ingestion');

    try {
        const resp = await fetch('/transactions/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: formData.toString()
        });

        if (resp.ok) {
            window.location.href = '/transactions?success=Transaction+saved+to+ledger';
        } else {
            alert('Failed to save transaction');
        }
    } catch (err) {
        alert('Error saving transaction: ' + err.message);
    }
};

// AI Interactive Financial Advisor Chat Handler
window.sendAIChatMessage = async function() {
    const input = document.getElementById('chatInput');
    const query = input.value.trim();
    if (!query) return;

    appendChatMessage('user', query);
    input.value = '';

    const typingId = appendChatMessage('assistant', 'Analyzing financial model with AI...');

    try {
        const resp = await fetch('/api/ai/advisor', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ query: query })
        });

        const data = await resp.json();
        const msgDiv = document.getElementById(typingId);
        if (msgDiv) {
            if (!resp.ok) {
                msgDiv.querySelector('.bubble-content').textContent = data.error || 'Advisor request failed';
            } else {
                msgDiv.querySelector('.bubble-content').innerHTML = formatMarkdownAdvice(data.reply);
            }
        }
    } catch (err) {
        const msgDiv = document.getElementById(typingId);
        if (msgDiv) {
            msgDiv.querySelector('.bubble-content').textContent = 'Error processing advice: ' + err.message;
        }
    }
};

window.sendQuickPrompt = function(promptText) {
    const input = document.getElementById('chatInput');
    if (input) {
        input.value = promptText;
        window.sendAIChatMessage();
    }
};

function appendChatMessage(role, text) {
    const chatBody = document.getElementById('chatMessages');
    if (!chatBody) return '';

    const id = 'msg_' + Math.random().toString(36).slice(2, 11);
    const bubble = document.createElement('div');
    bubble.className = `chat-bubble ${role}`;
    bubble.id = id;

    const avatar = document.createElement('div');
    avatar.className = 'avatar';
    avatar.textContent = role === 'user' ? '👤' : '🤖';

    const content = document.createElement('div');
    content.className = 'bubble-content';
    if (role === 'user') {
        content.textContent = text;
    } else {
        content.innerHTML = formatMarkdownAdvice(text);
    }

    bubble.appendChild(avatar);
    bubble.appendChild(content);
    chatBody.appendChild(bubble);
    chatBody.scrollTop = chatBody.scrollHeight;
    return id;
}

function formatMarkdownAdvice(text) {
    if (!text) return '';
    return escapeHtml(text)
        .replace(/\n/g, '<br>')
        .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
        .replace(/`([^`]+)`/g, '<code class="font-mono">$1</code>');
}

function showEmptyChartState(canvas, message) {
    if (!canvas || !canvas.parentElement) return;
    canvas.style.display = 'none';
    const note = document.createElement('div');
    note.className = 'chart-empty-state';
    note.textContent = message;
    canvas.parentElement.appendChild(note);
}

// Chart Initialization with Refined SaaS Palette
async function initCharts() {
    const trendCtx = document.getElementById('monthlyTrendChart');
    const doughnutCtx = document.getElementById('categoryDoughnutChart');
    const velCtx = document.getElementById('analyticsVelocityChart');
    const catCtx = document.getElementById('analyticsCategoryChart');

    if (!trendCtx && !doughnutCtx && !velCtx && !catCtx) return;

    try {
        const resTrends = await fetch('/api/reports/monthly');
        const trendsData = resTrends.ok ? await resTrends.json() : [];
        const trends = Array.isArray(trendsData) ? trendsData : [];

        const resCats = await fetch('/api/reports/categories');
        const catsData = resCats.ok ? await resCats.json() : [];
        const categories = Array.isArray(catsData) ? catsData : [];

        const cleanPalette = ['#6366f1', '#10b981', '#0ea5e9', '#f59e0b', '#ec4899', '#8b5cf6', '#06b6d4', '#84cc16', '#64748b'];

        // Render Dashboard Trends Chart
        if (trendCtx) {
            if (trends.length > 0) {
                new Chart(trendCtx, {
                    type: 'line',
                    data: {
                        labels: trends.map(t => t.month),
                        datasets: [
                            {
                                label: 'Income ($)',
                                data: trends.map(t => t.income),
                                borderColor: '#10b981',
                                backgroundColor: 'rgba(16, 185, 129, 0.08)',
                                borderWidth: 2.5,
                                pointBackgroundColor: '#10b981',
                                pointRadius: 3,
                                tension: 0.3,
                                fill: true
                            },
                            {
                                label: 'Expenses ($)',
                                data: trends.map(t => t.expense),
                                borderColor: '#6366f1',
                                backgroundColor: 'rgba(99, 102, 241, 0.08)',
                                borderWidth: 2.5,
                                pointBackgroundColor: '#6366f1',
                                pointRadius: 3,
                                tension: 0.3,
                                fill: true
                            }
                        ]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: {
                            legend: { labels: { color: '#94a3b8', font: { family: 'Inter', size: 12 } } }
                        },
                        scales: {
                            x: { ticks: { color: '#64748b', font: { family: 'Inter' } }, grid: { color: 'rgba(255, 255, 255, 0.03)' } },
                            y: { ticks: { color: '#64748b', font: { family: 'Inter' } }, grid: { color: 'rgba(255, 255, 255, 0.03)' } }
                        }
                    }
                });
            } else {
                showEmptyChartState(trendCtx, 'No monthly transaction trend data yet.');
            }
        }

        // Render Dashboard Category Doughnut
        if (doughnutCtx) {
            if (categories.length > 0) {
                new Chart(doughnutCtx, {
                    type: 'doughnut',
                    data: {
                        labels: categories.map(c => c.name),
                        datasets: [{
                            data: categories.map(c => c.amount),
                            backgroundColor: categories.map((c, i) => c.color || cleanPalette[i % cleanPalette.length]),
                            borderWidth: 2,
                            borderColor: '#141720'
                        }]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: {
                            legend: { position: 'bottom', labels: { color: '#94a3b8', font: { family: 'Inter', size: 11 } } }
                        }
                    }
                });
            } else {
                showEmptyChartState(doughnutCtx, 'Add expense transactions to see category allocation.');
            }
        }

        // Render Analytics Velocity Chart
        if (velCtx) {
            if (trends.length > 0) {
                new Chart(velCtx, {
                    type: 'bar',
                    data: {
                        labels: trends.map(t => t.month),
                        datasets: [{
                            label: 'Net Surplus ($)',
                            data: trends.map(t => t.savings),
                            backgroundColor: trends.map(t => t.savings >= 0 ? 'rgba(16, 185, 129, 0.8)' : 'rgba(239, 68, 68, 0.8)'),
                            borderRadius: 4
                        }]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: { legend: { display: false } },
                        scales: {
                            x: { ticks: { color: '#64748b' }, grid: { color: 'rgba(255,255,255,0.03)' } },
                            y: { ticks: { color: '#64748b' }, grid: { color: 'rgba(255,255,255,0.03)' } }
                        }
                    }
                });
            } else {
                showEmptyChartState(velCtx, 'No velocity data available yet.');
            }
        }

        // Render Analytics Category Horizontal Bar Chart
        if (catCtx) {
            if (categories.length > 0) {
                new Chart(catCtx, {
                    type: 'bar',
                    data: {
                        labels: categories.map(c => c.name),
                        datasets: [{
                            label: 'Spend ($)',
                            data: categories.map(c => c.amount),
                            backgroundColor: categories.map((c, i) => c.color || cleanPalette[i % cleanPalette.length]),
                            borderRadius: 4
                        }]
                    },
                    options: {
                        indexAxis: 'y',
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: { legend: { display: false } },
                        scales: {
                            x: { ticks: { color: '#64748b' }, grid: { color: 'rgba(255,255,255,0.03)' } },
                            y: { ticks: { color: '#64748b' }, grid: { color: 'rgba(255,255,255,0.03)' } }
                        }
                    }
                });
            } else {
                showEmptyChartState(catCtx, 'No category spend breakdown yet.');
            }
        }
    } catch (err) {
        console.error('Failed to initialize Enterprise charts:', err);
    }
}
