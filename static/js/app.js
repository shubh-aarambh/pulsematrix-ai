// PulseMatrix v7.0 — Frontend Controller

document.addEventListener('DOMContentLoaded', () => {
    syncStudioControls();
    initPersonalGreeting();
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
// Appearance Drawer (5 Curated Themes • 3 Textures • 2 Densities • Personal Note)
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
    localStorage.setItem('pm_theme_v7', theme);
    syncStudioControls();
    rebuildChartsForTheme();
};

window.setMatrixPattern = function(pattern) {
    document.documentElement.setAttribute('data-grid', pattern);
    localStorage.setItem('pm_grid_v7', pattern);
    syncStudioControls();
};

window.setDensityMode = function(density) {
    document.documentElement.setAttribute('data-density', density);
    localStorage.setItem('pm_density_v7', density);
    syncStudioControls();
};

window.savePersonalMotto = function(text) {
    localStorage.setItem('pm_motto_v7', text);
    initPersonalGreeting();
};

window.resetStudioDefaults = function() {
    localStorage.removeItem('pm_theme_v7');
    localStorage.removeItem('pm_grid_v7');
    localStorage.removeItem('pm_density_v7');
    localStorage.removeItem('pm_motto_v7');
    document.documentElement.setAttribute('data-theme', 'obsidian');
    document.documentElement.setAttribute('data-grid', 'clean');
    document.documentElement.setAttribute('data-density', 'comfortable');
    syncStudioControls();
    initPersonalGreeting();
    rebuildChartsForTheme();
};

function syncStudioControls() {
    const theme = document.documentElement.getAttribute('data-theme') || 'obsidian';
    const grid = document.documentElement.getAttribute('data-grid') || 'clean';
    const density = document.documentElement.getAttribute('data-density') || 'comfortable';

    document.querySelectorAll('[data-set-theme]').forEach(el => {
        el.classList.toggle('active', el.dataset.setTheme === theme);
    });
    document.querySelectorAll('[data-set-grid]').forEach(el => {
        el.classList.toggle('active', el.dataset.setGrid === grid);
    });
    document.querySelectorAll('[data-set-density]').forEach(el => {
        el.classList.toggle('active', el.dataset.setDensity === density);
    });

    const mottoInput = document.getElementById('studioMottoInput');
    if (mottoInput && mottoInput !== document.activeElement) {
        mottoInput.value = localStorage.getItem('pm_motto_v7') || '';
    }
}

function initPersonalGreeting() {
    const greetEl = document.getElementById('personalGreetingText');
    const mottoEl = document.getElementById('personalMottoText');
    const user = window.__PM_USER || 'Member';
    const curr = getCurrencyMeta();
    if (greetEl) {
        const h = new Date().getHours();
        const salutation = h < 12 ? 'Good morning' : (h < 17 ? 'Good afternoon' : 'Good evening');
        greetEl.textContent = `${salutation}, ${user}`;
    }
    if (mottoEl) {
        const saved = (localStorage.getItem('pm_motto_v7') || '').trim();
        mottoEl.textContent = saved || `Here is your financial overview in ${curr.code}.`;
    }
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
        { label: 'Overview', hint: 'Page', action: () => window.location.href = '/' },
        { label: 'Transactions', hint: 'Page', action: () => window.location.href = '/transactions' },
        { label: 'Budgets & Goals', hint: 'Page', action: () => window.location.href = '/goals' },
        { label: 'Shared Bills', hint: 'Page', action: () => window.location.href = '/splits' },
        { label: 'Net Worth', hint: 'Page', action: () => window.location.href = '/networth' },
        { label: 'Subscriptions', hint: 'Page', action: () => window.location.href = '/subscriptions' },
        { label: 'Analytics', hint: 'Page', action: () => window.location.href = '/analytics' },
        { label: 'AI Advisor', hint: 'Page', action: () => window.location.href = '/ai-advisor' },
        { label: 'Integrations', hint: 'Page', action: () => window.location.href = '/bank-sync' },
        { label: 'Monthly Statement (PDF)', hint: 'Page', action: () => window.location.href = '/report' },
        { label: 'Record expense', hint: 'Action', action: () => { closeCommandPalette(); openAddTxModal(); } },
        { label: 'Scan receipt or bank SMS', hint: 'Action', action: () => { closeCommandPalette(); openReceiptModal(); } },
        { label: 'Currency converter', hint: 'Tool', action: () => { closeCommandPalette(); openFXModal(); } },
        { label: 'Appearance & theme settings', hint: 'Settings', action: () => { closeCommandPalette(); openStudioDrawer(); } },
        { label: 'Export transactions CSV', hint: 'Download', action: () => window.location.href = '/transactions/export/csv' },
        { label: 'Switch currency to INR (₹)', hint: 'Currency', action: () => switchCurrencyQuick('INR') },
        { label: 'Switch currency to USD ($)', hint: 'Currency', action: () => switchCurrencyQuick('USD') },
        { label: 'Switch currency to EUR (€)', hint: 'Currency', action: () => switchCurrencyQuick('EUR') },
        { label: 'Switch currency to GBP (£)', hint: 'Currency', action: () => switchCurrencyQuick('GBP') },
    ];
    if (window.__PM_IS_ADMIN) {
        cmds.unshift({ label: 'Owner Console', hint: 'Owner', action: () => window.location.href = '/admin' });
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
        list.innerHTML = '<div class="cmd-empty">No matching results.</div>';
        return;
    }
    matches.forEach((cmd) => {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'cmd-item';
        btn.innerHTML = `<span class="cmd-item-left"><span>${escapeHtml(cmd.label)}</span></span><span class="cmd-hint">${escapeHtml(cmd.hint)}</span>`;
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

// Currency Converter
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
    metaEl.textContent = `1 ${from} = ${rate.toFixed(4)} ${to}`;
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
    const rawDate = (btn.dataset.date || '').trim();
    document.getElementById('editTxDate').value = rawDate.length >= 10 ? rawDate.slice(0, 10) : rawDate;
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

// Receipt / SMS Parser
window.runAIReceiptParser = async function() {
    const rawText = document.getElementById('aiRawText').value.trim();
    if (!rawText) { alert('Please enter receipt or SMS text.'); return; }

    const btnRun = document.getElementById('btnRunAI');
    const previewBox = document.getElementById('aiParsePreview');
    btnRun.textContent = 'Extracting…';
    btnRun.disabled = true;

    try {
        const resp = await fetch('/api/ai/parse-receipt', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ text: rawText })
        });
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
        const data = await resp.json();
        currentAIParsed = data;

        const confPct = ((Number(data.confidenceScore) || 0.85) * 100).toFixed(0);
        const amtNum = Number(data.amount) || 0;

        previewBox.classList.remove('hidden');
        previewBox.innerHTML = `
            <div style="font-weight:600; margin-bottom:0.45rem;">Extracted details (${escapeHtml(confPct)}% confidence)</div>
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:0.45rem; font-size:0.84rem;">
                <div><span class="text-muted">Merchant:</span> <strong>${escapeHtml(data.vendor)}</strong></div>
                <div><span class="text-muted">Amount:</span> <strong class="font-mono">${escapeHtml(amtNum.toFixed(2))} ${escapeHtml(data.currency)}</strong></div>
                <div><span class="text-muted">Category:</span> ${escapeHtml(data.suggestedCategory)}</div>
                <div><span class="text-muted">Date:</span> <span class="font-mono">${escapeHtml(data.date)}</span></div>
            </div>
            ${data.isAnomaly ? `<div class="badge badge-warning" style="margin-top:0.5rem;">Flagged: ${escapeHtml(data.anomalyReason)}</div>` : ''}
        `;

        document.getElementById('btnConfirmAI').classList.remove('hidden');
        btnRun.classList.add('hidden');
    } catch (err) {
        alert('Could not parse text: ' + err.message);
    } finally {
        btnRun.textContent = 'Extract details';
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
    formData.append('description', 'Scanned receipt');

    try {
        const resp = await fetch('/transactions/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: formData.toString()
        });
        if (resp.ok) window.location.href = '/transactions?success=Transaction+saved';
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
    const typingId = appendChatMessage('assistant', 'Reviewing your ledger…');

    try {
        const resp = await fetch('/api/ai/advisor', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ query: query })
        });
        const data = await resp.json();
        const msgDiv = document.getElementById(typingId);
        if (msgDiv) {
            if (!resp.ok) msgDiv.querySelector('.bubble-content').textContent = data.error || 'Request failed';
            else msgDiv.querySelector('.bubble-content').innerHTML = formatMarkdownAdvice(data.reply);
        }
    } catch (err) {
        const msgDiv = document.getElementById(typingId);
        if (msgDiv) msgDiv.querySelector('.bubble-content').textContent = 'Error: ' + err.message;
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
    avatar.textContent = role === 'user' ? 'You' : 'PM';

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

// Clean Editorial Chart.js Engine
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
        console.error('Chart init error:', err);
    }
}

function renderAllCharts(trends, categories) {
    const trendCtx = document.getElementById('monthlyTrendChart');
    const doughnutCtx = document.getElementById('categoryDoughnutChart');
    const velCtx = document.getElementById('analyticsVelocityChart');
    const catCtx = document.getElementById('analyticsCategoryChart');

    const curr = getCurrencyMeta();
    const sym = curr.symbol || '₹';
    const isLight = document.documentElement.getAttribute('data-theme') === 'alabaster';

    const incomeColor = cssVar('--accent-success', '#22c55e');
    const expenseColor = cssVar('--accent-primary', '#3b82f6');
    const tickColor = isLight ? '#6b7280' : '#71717a';
    const legendColor = isLight ? '#374151' : '#a1a1aa';
    const gridColor = isLight ? 'rgba(17, 24, 39, 0.06)' : 'rgba(255, 255, 255, 0.06)';
    const cardBg = cssVar('--bg-card-solid', '#121215');

    const palette = ['#3b82f6', '#22c55e', '#f59e0b', '#818cf8', '#06b6d4', '#ec4899', '#84cc16', '#64748b'];

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
                            borderColor: incomeColor,
                            backgroundColor: 'rgba(34, 197, 94, 0.08)',
                            borderWidth: 2,
                            pointBackgroundColor: incomeColor,
                            pointRadius: 3,
                            tension: 0.3,
                            fill: true
                        },
                        {
                            label: `Expenses (${sym})`,
                            data: trends.map(t => t.expense),
                            borderColor: expenseColor,
                            backgroundColor: 'rgba(59, 130, 246, 0.08)',
                            borderWidth: 2,
                            pointBackgroundColor: expenseColor,
                            pointRadius: 3,
                            tension: 0.3,
                            fill: true
                        }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: { legend: { labels: { color: legendColor, boxWidth: 10, font: { family: 'Inter', size: 12 } } } },
                    scales: {
                        x: { ticks: { color: tickColor, font: { family: 'JetBrains Mono', size: 11 } }, grid: { display: false } },
                        y: { ticks: { color: tickColor, font: { family: 'JetBrains Mono', size: 11 }, callback: (v) => sym + Number(v).toLocaleString() }, grid: { color: gridColor } }
                    }
                }
            }));
        } else {
            showEmptyChartState(trendCtx, 'No monthly transaction data yet.');
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
                        backgroundColor: categories.map((_, i) => palette[i % palette.length]),
                        borderWidth: 2,
                        borderColor: cardBg
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    cutout: '68%',
                    plugins: { legend: { position: 'bottom', labels: { color: legendColor, boxWidth: 10, font: { family: 'Inter', size: 11 } } } }
                }
            }));
        } else {
            showEmptyChartState(doughnutCtx, 'Record expenses to see category distribution.');
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
                        backgroundColor: trends.map(t => t.savings >= 0 ? 'rgba(34, 197, 94, 0.8)' : 'rgba(239, 68, 68, 0.8)'),
                        borderRadius: 5
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: { legend: { display: false } },
                    scales: {
                        x: { ticks: { color: tickColor, font: { family: 'JetBrains Mono', size: 11 } }, grid: { display: false } },
                        y: { ticks: { color: tickColor, font: { family: 'JetBrains Mono', size: 11 }, callback: (v) => sym + Number(v).toLocaleString() }, grid: { color: gridColor } }
                    }
                }
            }));
        } else {
            showEmptyChartState(velCtx, 'No monthly surplus data yet.');
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
                        backgroundColor: categories.map((_, i) => palette[i % palette.length]),
                        borderRadius: 5
                    }]
                },
                options: {
                    indexAxis: 'y',
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: { legend: { display: false } },
                    scales: {
                        x: { ticks: { color: tickColor, font: { family: 'JetBrains Mono', size: 11 }, callback: (v) => sym + Number(v).toLocaleString() }, grid: { color: gridColor } },
                        y: { ticks: { color: tickColor, font: { family: 'Inter', size: 12 } }, grid: { display: false } }
                    }
                }
            }));
        } else {
            showEmptyChartState(catCtx, 'No category breakdown yet.');
        }
    }
}
