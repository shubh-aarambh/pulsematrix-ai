// PulseMatrix AI v6.0 — Personalized Matrix Bento Wealth OS Controller

document.addEventListener('DOMContentLoaded', () => {
    syncStudioControls();
    initPersonalGreeting();
    initCursorSpotlight();
    initCharts();
    initModalUX();
    initAutoDismissToasts();
    initKeyboardShortcuts();
});

let currentAIParsed = null;
let fxRatesCache = null;
const chartInstances = [];

function escapeHtml(str) {
    if (str === null || str === undefined) return '';
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

function getCurrencyMeta() {
    return window.__PM_CURRENCY || { code: 'INR', symbol: '₹', rate: 1 };
}

function cssVar(name, fallback) {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback;
}

// ============================================================================
// Personalization Studio (6 Themes • 4 Matrix Patterns • 3 Glows • Motto)
// ============================================================================

window.openStudioDrawer = function() {
    document.getElementById('studioBackdrop')?.classList.add('open');
    document.getElementById('studioDrawer')?.classList.add('open');
    syncStudioControls();
};

window.closeStudioDrawer = function() {
    document.getElementById('studioBackdrop')?.classList.remove('open');
    document.getElementById('studioDrawer')?.classList.remove('open');
};

window.setThemePalette = function(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('pm_theme', theme);
    syncStudioControls();
    rebuildChartsForTheme();
};

window.setMatrixPattern = function(pattern) {
    document.documentElement.setAttribute('data-grid', pattern);
    localStorage.setItem('pm_grid', pattern);
    syncStudioControls();
};

window.setGlowLevel = function(glow) {
    document.documentElement.setAttribute('data-glow', glow);
    localStorage.setItem('pm_glow', glow);
    syncStudioControls();
};

window.setDensityMode = function(density) {
    document.documentElement.setAttribute('data-density', density);
    localStorage.setItem('pm_density', density);
    syncStudioControls();
};

window.savePersonalMotto = function(text) {
    localStorage.setItem('pm_motto', text);
    initPersonalGreeting();
};

window.resetStudioDefaults = function() {
    localStorage.removeItem('pm_theme');
    localStorage.removeItem('pm_grid');
    localStorage.removeItem('pm_glow');
    localStorage.removeItem('pm_density');
    localStorage.removeItem('pm_motto');
    document.documentElement.setAttribute('data-theme', 'cyber');
    document.documentElement.setAttribute('data-grid', 'blueprint');
    document.documentElement.setAttribute('data-glow', 'vibrant');
    document.documentElement.setAttribute('data-density', 'comfortable');
    syncStudioControls();
    initPersonalGreeting();
    rebuildChartsForTheme();
};

function syncStudioControls() {
    const theme = document.documentElement.getAttribute('data-theme') || 'cyber';
    const grid = document.documentElement.getAttribute('data-grid') || 'blueprint';
    const glow = document.documentElement.getAttribute('data-glow') || 'vibrant';
    const density = document.documentElement.getAttribute('data-density') || 'comfortable';

    document.querySelectorAll('[data-set-theme]').forEach(el => {
        el.classList.toggle('active', el.dataset.setTheme === theme);
    });
    document.querySelectorAll('[data-set-grid]').forEach(el => {
        el.classList.toggle('active', el.dataset.setGrid === grid);
    });
    document.querySelectorAll('[data-set-glow]').forEach(el => {
        el.classList.toggle('active', el.dataset.setGlow === glow);
    });
    document.querySelectorAll('[data-set-density]').forEach(el => {
        el.classList.toggle('active', el.dataset.setDensity === density);
    });

    const mottoInput = document.getElementById('studioMottoInput');
    if (mottoInput && mottoInput !== document.activeElement) {
        mottoInput.value = localStorage.getItem('pm_motto') || '';
    }
}

function initPersonalGreeting() {
    const greetEl = document.getElementById('personalGreetingText');
    const mottoEl = document.getElementById('personalMottoText');
    const user = window.__PM_USER || 'Member';
    if (greetEl) {
        const h = new Date().getHours();
        const salutation = h < 12 ? 'Good morning' : (h < 17 ? 'Good afternoon' : 'Good evening');
        greetEl.textContent = `${salutation}, ${user}`;
    }
    if (mottoEl) {
        const savedMotto = (localStorage.getItem('pm_motto') || '').trim();
        mottoEl.textContent = savedMotto || 'Your autonomous wealth & cashflow telemetry is synced in real time.';
    }
}

// Smooth Mouse-Tracking Spotlight Glow on Bento Cards
function initCursorSpotlight() {
    document.querySelectorAll('.card').forEach((card) => {
        card.addEventListener('pointermove', (e) => {
            const rect = card.getBoundingClientRect();
            card.style.setProperty('--mx', `${e.clientX - rect.left}px`);
            card.style.setProperty('--my', `${e.clientY - rect.top}px`);
        });
    });
}

function switchCurrencyQuick(code) {
    const form = document.createElement('form');
    form.method = 'POST';
    form.action = '/settings/currency';
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = 'currency';
    input.value = code;
    form.appendChild(input);
    document.body.appendChild(form);
    form.submit();
}

// Command Palette (Ctrl+K / Cmd+K)
function getCommands() {
    const cmds = [
        { label: 'Studio: Personalize Theme, Matrix Grid & Glow', hint: 'Colors', icon: '🎨', action: () => { closeCommandPalette(); openStudioDrawer(); } },
        { label: 'Go to Command Center', hint: 'Bento Dashboard', icon: '📊', action: () => window.location.href = '/' },
        { label: 'Go to Transactions Ledger', hint: 'Edit & CSV', icon: '💳', action: () => window.location.href = '/transactions' },
        { label: 'Go to Budgets & Savings Goals', hint: 'Envelopes', icon: '🎯', action: () => window.location.href = '/goals' },
        { label: 'Go to Split-Bills & Shared Ledger', hint: 'IOUs', icon: '🤝', action: () => window.location.href = '/splits' },
        { label: 'Go to Net Worth & Balance Sheet', hint: 'Assets/Debt', icon: '💎', action: () => window.location.href = '/networth' },
        { label: 'Go to Monthly Statement (PDF)', hint: 'Print', icon: '📄', action: () => window.location.href = '/report' },
        { label: 'Go to Analytics & Risk Radar', hint: 'Heatmap', icon: '📈', action: () => window.location.href = '/analytics' },
        { label: 'Go to Subscriptions Radar', hint: 'AutoPay', icon: '🔄', action: () => window.location.href = '/subscriptions' },
        { label: 'Go to AI Intelligence Strategist', hint: 'Chat', icon: '🤖', action: () => window.location.href = '/ai-advisor' },
        { label: 'Go to UPI & Webhook Integrations', hint: 'SMS Sync', icon: '📱', action: () => window.location.href = '/bank-sync' },
        { label: 'Currency: Switch Display to INR (₹)', hint: 'Default', icon: '🇮🇳', action: () => switchCurrencyQuick('INR') },
        { label: 'Currency: Switch Display to USD ($)', hint: 'Live FX', icon: '🇺🇸', action: () => switchCurrencyQuick('USD') },
        { label: 'Currency: Switch Display to EUR (€)', hint: 'Live FX', icon: '🇪🇺', action: () => switchCurrencyQuick('EUR') },
        { label: 'Currency: Switch Display to GBP (£)', hint: 'Live FX', icon: '🇬🇧', action: () => switchCurrencyQuick('GBP') },
        { label: 'Action: Add New Transaction', hint: 'Modal', icon: '➕', action: () => { closeCommandPalette(); openAddTxModal(); } },
        { label: 'Action: Scan Receipt / UPI SMS with AI', hint: 'NLP', icon: '📷', action: () => { closeCommandPalette(); openReceiptModal(); } },
        { label: 'Action: Open Live FX Calculator', hint: 'Convert', icon: '💱', action: () => { closeCommandPalette(); openFXModal(); } },
        { label: 'Action: Export Full Ledger CSV', hint: 'Download', icon: '📥', action: () => window.location.href = '/transactions/export/csv' },
    ];
    if (window.__PM_IS_ADMIN) {
        cmds.unshift({ label: 'Go to Owner Control Plane', hint: 'Telemetry', icon: '🛡️', action: () => window.location.href = '/admin' });
    }
    return cmds;
}

window.openCommandPalette = function() {
    const modal = document.getElementById('commandPaletteModal');
    const input = document.getElementById('cmdSearchInput');
    if (!modal) return;
    modal.style.display = 'flex';
    if (input) {
        input.value = '';
        filterCommandPalette('');
        setTimeout(() => input.focus(), 30);
    }
};

window.closeCommandPalette = function() {
    const modal = document.getElementById('commandPaletteModal');
    if (modal) modal.style.display = 'none';
};

window.filterCommandPalette = function(q) {
    const list = document.getElementById('cmdList');
    if (!list) return;
    const term = (q || '').trim().toLowerCase();
    const matches = getCommands().filter(c =>
        !term || c.label.toLowerCase().includes(term) || c.hint.toLowerCase().includes(term)
    );
    list.innerHTML = '';
    if (matches.length === 0) {
        list.innerHTML = '<div class="cmd-empty">No matching commands found.</div>';
        return;
    }
    matches.forEach((cmd) => {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'cmd-item';
        btn.innerHTML = `<span class="cmd-item-left"><span>${escapeHtml(cmd.icon)}</span><span>${escapeHtml(cmd.label)}</span></span><span class="cmd-hint">${escapeHtml(cmd.hint)}</span>`;
        btn.addEventListener('click', cmd.action);
        list.appendChild(btn);
    });
};

function initKeyboardShortcuts() {
    document.addEventListener('keydown', (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
            e.preventDefault();
            openCommandPalette();
        }
    });
}

// Live Multi-Currency FX Converter
window.openFXModal = async function() {
    const modal = document.getElementById('fxConverterModal');
    if (!modal) return;
    modal.style.display = 'flex';
    if (!fxRatesCache) {
        try {
            const resp = await fetch('/api/currency/rates');
            if (resp.ok) {
                const data = await resp.json();
                fxRatesCache = data.rates || {};
            }
        } catch (err) {
            console.error('FX rates fetch error:', err);
        }
    }
    computeFXConversion();
};

window.closeFXModal = function() {
    const modal = document.getElementById('fxConverterModal');
    if (modal) modal.style.display = 'none';
};

window.swapFXCurrencies = function() {
    const fromEl = document.getElementById('fxFrom');
    const toEl = document.getElementById('fxTo');
    if (!fromEl || !toEl) return;
    const tmp = fromEl.value;
    fromEl.value = toEl.value;
    toEl.value = tmp;
    computeFXConversion();
};

window.computeFXConversion = function() {
    const amt = parseFloat(document.getElementById('fxAmount')?.value || '0') || 0;
    const from = document.getElementById('fxFrom')?.value || 'INR';
    const to = document.getElementById('fxTo')?.value || 'USD';
    const valEl = document.getElementById('fxResultValue');
    const metaEl = document.getElementById('fxRateMeta');
    if (!valEl || !metaEl) return;

    const rates = fxRatesCache || { USD: 1, EUR: 0.92, GBP: 0.79, INR: 84.5, CAD: 1.38, AUD: 1.53, JPY: 154.0 };
    const rFrom = rates[from] || 1;
    const rTo = rates[to] || 1;
    const rate = rTo / rFrom;
    const converted = amt * rate;

    valEl.textContent = `${amt.toLocaleString(undefined, { maximumFractionDigits: 2 })} ${from} = ${converted.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ${to}`;
    metaEl.textContent = `Live Rate: 1 ${from} = ${rate.toFixed(4)} ${to}`;
};

// Edit Transaction & CSV Import Modals
window.openEditTxModal = function(btn) {
    const modal = document.getElementById('editTxModal');
    if (!modal || !btn) return;
    document.getElementById('editTxId').value = btn.dataset.id || '';
    document.getElementById('editTxVendor').value = btn.dataset.vendor || '';
    document.getElementById('editTxAmount').value = btn.dataset.amount || '';
    document.getElementById('editTxCurrency').value = btn.dataset.currency || 'INR';
    document.getElementById('editTxCategory').value = btn.dataset.categoryId || '';
    document.getElementById('editTxDate').value = btn.dataset.date || '';
    document.getElementById('editTxMethod').value = btn.dataset.method || '';
    document.getElementById('editTxDescription').value = btn.dataset.description || '';
    modal.style.display = 'flex';
};

window.closeEditTxModal = function() {
    const modal = document.getElementById('editTxModal');
    if (modal) modal.style.display = 'none';
};

window.openImportCsvModal = function() {
    const modal = document.getElementById('importCsvModal');
    if (modal) modal.style.display = 'flex';
};

window.closeImportCsvModal = function() {
    const modal = document.getElementById('importCsvModal');
    if (modal) modal.style.display = 'none';
};

function initModalUX() {
    document.querySelectorAll('.modal-backdrop').forEach((backdrop) => {
        backdrop.addEventListener('click', (e) => {
            if (e.target === backdrop) backdrop.style.display = 'none';
        });
    });

    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            document.querySelectorAll('.modal-backdrop').forEach((m) => { m.style.display = 'none'; });
            closeStudioDrawer();
            document.querySelector('.sidebar')?.classList.remove('open');
        }
    });
}

function initAutoDismissToasts() {
    const toast = document.getElementById('flash-alert');
    if (toast) {
        setTimeout(() => { if (toast.parentNode) toast.remove(); }, 5000);
    }
}

window.toggleMobileSidebar = function() {
    document.querySelector('.sidebar')?.classList.toggle('open');
};

window.openReceiptModal = function() {
    const modal = document.getElementById('aiReceiptModal');
    if (modal) modal.style.display = 'flex';
};

window.closeReceiptModal = function() {
    const modal = document.getElementById('aiReceiptModal');
    if (modal) modal.style.display = 'none';
    document.getElementById('aiParsePreview')?.classList.add('hidden');
    document.getElementById('btnConfirmAI')?.classList.add('hidden');
    document.getElementById('btnRunAI')?.classList.remove('hidden');
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
    if (modal) modal.style.display = (modal.style.display === 'flex') ? 'none' : 'flex';
};

// AI Receipt Parser
window.runAIReceiptParser = async function() {
    const rawText = document.getElementById('aiRawText').value.trim();
    if (!rawText) { alert('Please enter or paste receipt text.'); return; }

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
        if (!resp.ok) throw new Error(`Parser Error HTTP ${resp.status}`);
        const data = await resp.json();
        currentAIParsed = data;

        const confPct = ((Number(data.confidenceScore) || 0.85) * 100).toFixed(0);
        const amtNum = Number(data.amount) || 0;

        previewBox.classList.remove('hidden');
        previewBox.innerHTML = `
            <div style="font-weight:700; color:var(--accent-primary); margin-bottom:0.4rem;">Parsed Receipt Data (Confidence: ${escapeHtml(confPct)}%)</div>
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:0.5rem; font-size:0.86rem;">
                <div><strong>Merchant:</strong> ${escapeHtml(data.vendor)}</div>
                <div><strong>Amount:</strong> <span style="color:var(--accent-secondary); font-weight:700;">${escapeHtml(amtNum.toFixed(2))} ${escapeHtml(data.currency)}</span></div>
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

window.confirmAITransaction = async function() {
    if (!currentAIParsed) return;
    const formData = new URLSearchParams();
    formData.append('vendor', currentAIParsed.vendor || 'Merchant');
    formData.append('amount', currentAIParsed.amount || 0);
    formData.append('currency', currentAIParsed.currency || 'INR');
    formData.append('date', currentAIParsed.date || '');
    formData.append('category_name', currentAIParsed.suggestedCategory || 'Miscellaneous');
    formData.append('payment_method', currentAIParsed.paymentMethod || 'UPI');
    formData.append('source', 'ai_ocr');
    formData.append('description', 'AI Receipt Ingestion');

    try {
        const resp = await fetch('/transactions/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: formData.toString()
        });
        if (resp.ok) window.location.href = '/transactions?success=Transaction+saved+to+ledger';
        else alert('Failed to save transaction');
    } catch (err) {
        alert('Error saving transaction: ' + err.message);
    }
};

// AI Advisor Chat
window.sendAIChatMessage = async function() {
    const input = document.getElementById('chatInput');
    const query = input.value.trim();
    if (!query) return;

    appendChatMessage('user', query);
    input.value = '';
    const typingId = appendChatMessage('assistant', 'Analyzing financial telemetry...');

    try {
        const resp = await fetch('/api/ai/advisor', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ query: query })
        });
        const data = await resp.json();
        const msgDiv = document.getElementById(typingId);
        if (msgDiv) {
            if (!resp.ok) msgDiv.querySelector('.bubble-content').textContent = data.error || 'Advisor request failed';
            else msgDiv.querySelector('.bubble-content').innerHTML = formatMarkdownAdvice(data.reply);
        }
    } catch (err) {
        const msgDiv = document.getElementById(typingId);
        if (msgDiv) msgDiv.querySelector('.bubble-content').textContent = 'Error processing advice: ' + err.message;
    }
};

window.sendQuickPrompt = function(promptText) {
    const input = document.getElementById('chatInput');
    if (input) { input.value = promptText; window.sendAIChatMessage(); }
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
    if (role === 'user') content.textContent = text;
    else content.innerHTML = formatMarkdownAdvice(text);

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

// Live Theme-Aware Chart.js Engine
let cachedTrends = null;
let cachedCategories = null;

function rebuildChartsForTheme() {
    while (chartInstances.length > 0) {
        const ch = chartInstances.pop();
        try { ch.destroy(); } catch (_) {}
    }
    if (cachedTrends !== null || cachedCategories !== null) {
        renderAllCharts(cachedTrends || [], cachedCategories || []);
    }
}

async function initCharts() {
    const trendCtx = document.getElementById('monthlyTrendChart');
    const doughnutCtx = document.getElementById('categoryDoughnutChart');
    const velCtx = document.getElementById('analyticsVelocityChart');
    const catCtx = document.getElementById('analyticsCategoryChart');
    if (!trendCtx && !doughnutCtx && !velCtx && !catCtx) return;

    try {
        const [resTrends, resCats] = await Promise.all([
            fetch('/api/reports/monthly'),
            fetch('/api/reports/categories')
        ]);
        const tData = resTrends.ok ? await resTrends.json() : [];
        const cData = resCats.ok ? await resCats.json() : [];
        cachedTrends = Array.isArray(tData) ? tData : [];
        cachedCategories = Array.isArray(cData) ? cData : [];
        renderAllCharts(cachedTrends, cachedCategories);
    } catch (err) {
        console.error('Failed to initialize charts:', err);
    }
}

function renderAllCharts(trends, categories) {
    const trendCtx = document.getElementById('monthlyTrendChart');
    const doughnutCtx = document.getElementById('categoryDoughnutChart');
    const velCtx = document.getElementById('analyticsVelocityChart');
    const catCtx = document.getElementById('analyticsCategoryChart');

    const curr = getCurrencyMeta();
    const sym = curr.symbol || '₹';
    const primary = cssVar('--accent-primary', '#00f5a0');
    const primaryRGB = cssVar('--accent-primary-rgb', '0, 245, 160');
    const secondary = cssVar('--accent-secondary', '#00d9f5');
    const secondaryRGB = cssVar('--accent-secondary-rgb', '0, 217, 245');
    const isLight = document.documentElement.getAttribute('data-theme') === 'frost';

    const tickColor = isLight ? '#475569' : '#64748b';
    const legendColor = isLight ? '#334155' : '#94a3b8';
    const gridColor = isLight ? 'rgba(15, 23, 42, 0.06)' : `rgba(${primaryRGB}, 0.08)`;
    const doughnutBorder = cssVar('--bg-card-solid', '#0b1219');

    const themePalette = [primary, secondary, '#f59e0b', '#fb7185', '#14b8a6', '#38bdf8', '#eab308', '#22c55e', '#64748b'];

    if (trendCtx) {
        if (trends.length > 0) {
            chartInstances.push(new Chart(trendCtx, {
                type: 'line',
                data: {
                    labels: trends.map(t => t.month),
                    datasets: [
                        {
                            label: `Income (${sym})`,
                            data: trends.map(t => t.income),
                            borderColor: primary,
                            backgroundColor: `rgba(${primaryRGB}, 0.14)`,
                            borderWidth: 2.6,
                            pointBackgroundColor: primary,
                            pointRadius: 4,
                            tension: 0.36,
                            fill: true
                        },
                        {
                            label: `Expenses (${sym})`,
                            data: trends.map(t => t.expense),
                            borderColor: secondary,
                            backgroundColor: `rgba(${secondaryRGB}, 0.11)`,
                            borderWidth: 2.6,
                            pointBackgroundColor: secondary,
                            pointRadius: 4,
                            tension: 0.36,
                            fill: true
                        }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: { legend: { labels: { color: legendColor, font: { family: 'JetBrains Mono', size: 11 } } } },
                    scales: {
                        x: { ticks: { color: tickColor, font: { family: 'JetBrains Mono', size: 11 } }, grid: { color: gridColor } },
                        y: { ticks: { color: tickColor, font: { family: 'JetBrains Mono', size: 11 }, callback: (v) => sym + Number(v).toLocaleString() }, grid: { color: gridColor } }
                    }
                }
            }));
        } else {
            showEmptyChartState(trendCtx, 'No monthly transaction trend data yet.');
        }
    }

    if (doughnutCtx) {
        if (categories.length > 0) {
            chartInstances.push(new Chart(doughnutCtx, {
                type: 'doughnut',
                data: {
                    labels: categories.map(c => c.name),
                    datasets: [{
                        data: categories.map(c => c.amount),
                        backgroundColor: categories.map((_, i) => themePalette[i % themePalette.length]),
                        borderWidth: 2,
                        borderColor: doughnutBorder
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    cutout: '65%',
                    plugins: { legend: { position: 'bottom', labels: { color: legendColor, font: { family: 'Inter', size: 11 } } } }
                }
            }));
        } else {
            showEmptyChartState(doughnutCtx, 'Add expense transactions to see category allocation.');
        }
    }

    if (velCtx) {
        if (trends.length > 0) {
            chartInstances.push(new Chart(velCtx, {
                type: 'bar',
                data: {
                    labels: trends.map(t => t.month),
                    datasets: [{
                        label: `Net Surplus (${sym})`,
                        data: trends.map(t => t.savings),
                        backgroundColor: trends.map(t => t.savings >= 0 ? `rgba(${primaryRGB}, 0.85)` : 'rgba(251, 113, 133, 0.85)'),
                        borderRadius: 6
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: { legend: { display: false } },
                    scales: {
                        x: { ticks: { color: tickColor, font: { family: 'JetBrains Mono', size: 11 } }, grid: { color: gridColor } },
                        y: { ticks: { color: tickColor, font: { family: 'JetBrains Mono', size: 11 }, callback: (v) => sym + Number(v).toLocaleString() }, grid: { color: gridColor } }
                    }
                }
            }));
        } else {
            showEmptyChartState(velCtx, 'No velocity data available yet.');
        }
    }

    if (catCtx) {
        if (categories.length > 0) {
            chartInstances.push(new Chart(catCtx, {
                type: 'bar',
                data: {
                    labels: categories.map(c => c.name),
                    datasets: [{
                        label: `Spend (${sym})`,
                        data: categories.map(c => c.amount),
                        backgroundColor: categories.map((_, i) => themePalette[i % themePalette.length]),
                        borderRadius: 6
                    }]
                },
                options: {
                    indexAxis: 'y',
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: { legend: { display: false } },
                    scales: {
                        x: { ticks: { color: tickColor, font: { family: 'JetBrains Mono', size: 11 }, callback: (v) => sym + Number(v).toLocaleString() }, grid: { color: gridColor } },
                        y: { ticks: { color: tickColor }, grid: { color: gridColor } }
                    }
                }
            }));
        } else {
            showEmptyChartState(catCtx, 'No category spend breakdown yet.');
        }
    }
}
