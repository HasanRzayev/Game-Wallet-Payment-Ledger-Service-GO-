package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Game Wallet & Ledger — Modern Monetization Platform</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@400;500;600;700;800&family=JetBrains+Mono:wght@500;600;700&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg: #f8fafc;
      --surface: #ffffff;
      --surface-subtle: #f1f5f9;
      --border: #e2e8f0;
      --border-focus: #6366f1;
      --text-main: #0f172a;
      --text-secondary: #475569;
      --text-muted: #94a3b8;
      --primary: #4f46e5;
      --primary-hover: #4338ca;
      --primary-light: #eef2ff;
      --success: #059669;
      --success-hover: #047857;
      --success-light: #ecfdf5;
      --danger: #e11d48;
      --danger-hover: #be123c;
      --danger-light: #fff1f2;
      --warning: #d97706;
      --warning-hover: #b45309;
      --warning-light: #fffbeb;
      --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.05);
      --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.07), 0 2px 4px -2px rgba(0, 0, 0, 0.05);
      --shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.08), 0 4px 6px -4px rgba(0, 0, 0, 0.04);
      --radius: 14px;
    }

    * { box-sizing: border-box; margin: 0; padding: 0; }

    body {
      background-color: var(--bg);
      background-image: radial-gradient(#cbd5e1 0.75px, transparent 0.75px);
      background-size: 24px 24px;
      color: var(--text-main);
      font-family: 'Plus Jakarta Sans', -apple-system, BlinkMacSystemFont, sans-serif;
      min-height: 100vh;
      padding: 32px 24px;
      line-height: 1.5;
    }

    .container {
      max-width: 1320px;
      margin: 0 auto;
    }

    /* Top Navigation Bar */
    .navbar {
      background: rgba(255, 255, 255, 0.92);
      backdrop-filter: blur(12px);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      padding: 18px 28px;
      display: flex;
      justify-content: space-between;
      align-items: center;
      box-shadow: var(--shadow-sm);
      margin-bottom: 28px;
    }

    .brand {
      display: flex;
      align-items: center;
      gap: 14px;
    }

    .brand-icon {
      width: 44px;
      height: 44px;
      background: linear-gradient(135deg, #4f46e5, #06b6d4);
      border-radius: 12px;
      display: flex;
      align-items: center;
      justify-content: center;
      color: #ffffff;
      font-size: 22px;
      box-shadow: 0 4px 10px rgba(79, 70, 229, 0.3);
    }

    .brand-text h1 {
      font-size: 20px;
      font-weight: 800;
      color: var(--text-main);
      letter-spacing: -0.5px;
    }

    .brand-text p {
      font-size: 13px;
      color: var(--text-secondary);
      font-weight: 500;
    }

    .status-pill {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      background: var(--success-light);
      border: 1px solid #a7f3d0;
      padding: 6px 14px;
      border-radius: 9999px;
      font-size: 13px;
      font-weight: 600;
      color: var(--success);
    }

    .status-pulse {
      width: 8px;
      height: 8px;
      background-color: var(--success);
      border-radius: 50%;
      box-shadow: 0 0 0 0 rgba(5, 150, 105, 0.5);
      animation: pulse 2s infinite;
    }

    @keyframes pulse {
      0% { box-shadow: 0 0 0 0 rgba(5, 150, 105, 0.7); }
      70% { box-shadow: 0 0 0 6px rgba(5, 150, 105, 0); }
      100% { box-shadow: 0 0 0 0 rgba(5, 150, 105, 0); }
    }

    /* Hero Overview Card */
    .hero-banner {
      background: linear-gradient(135deg, #ffffff 0%, #f8fafc 100%);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      padding: 28px 32px;
      box-shadow: var(--shadow-sm);
      margin-bottom: 28px;
      display: grid;
      grid-template-columns: 1.2fr 2fr;
      gap: 28px;
      align-items: center;
    }

    .balance-card {
      background: linear-gradient(135deg, #4f46e5 0%, #3b82f6 100%);
      border-radius: 16px;
      padding: 24px;
      color: #ffffff;
      box-shadow: 0 10px 25px -5px rgba(79, 70, 229, 0.4);
      position: relative;
      overflow: hidden;
    }

    .balance-card::after {
      content: '';
      position: absolute;
      top: -30%;
      right: -20%;
      width: 180px;
      height: 180px;
      background: rgba(255, 255, 255, 0.12);
      border-radius: 50%;
      pointer-events: none;
    }

    .balance-label {
      font-size: 13px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 1px;
      color: #c7d2fe;
      margin-bottom: 8px;
    }

    .balance-amount {
      font-size: 42px;
      font-weight: 800;
      font-family: 'JetBrains Mono', monospace;
      letter-spacing: -1px;
      line-height: 1;
      margin-bottom: 12px;
    }

    .balance-cents {
      font-size: 13px;
      color: #e0e7ff;
      font-weight: 500;
    }

    .meta-details {
      display: grid;
      grid-template-columns: repeat(2, 1fr);
      gap: 16px;
    }

    .meta-item {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 14px 18px;
    }

    .meta-item-label {
      font-size: 11px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: var(--text-muted);
      margin-bottom: 4px;
    }

    .meta-item-value {
      font-size: 14px;
      font-weight: 700;
      color: var(--text-main);
      font-family: 'JetBrains Mono', monospace;
      word-break: break-all;
    }

    /* Actions Grid */
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(290px, 1fr));
      gap: 20px;
      margin-bottom: 28px;
    }

    .card {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      padding: 24px;
      box-shadow: var(--shadow-sm);
      display: flex;
      flex-direction: column;
      transition: transform 0.2s, box-shadow 0.2s;
    }

    .card:hover {
      box-shadow: var(--shadow-md);
      transform: translateY(-2px);
    }

    .card-header {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 18px;
    }

    .card-icon {
      width: 36px;
      height: 36px;
      border-radius: 10px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 18px;
    }

    .icon-blue { background: #e0e7ff; color: #4338ca; }
    .icon-green { background: #d1fae5; color: #047857; }
    .icon-rose { background: #ffe4e6; color: #be123c; }
    .icon-amber { background: #fef3c7; color: #b45309; }

    .card-header h2 {
      font-size: 16px;
      font-weight: 700;
      color: var(--text-main);
    }

    .form-group {
      margin-bottom: 14px;
    }

    label {
      display: block;
      font-size: 12px;
      font-weight: 700;
      color: var(--text-secondary);
      margin-bottom: 6px;
    }

    input {
      width: 100%;
      background: #ffffff;
      border: 1.5px solid var(--border);
      color: var(--text-main);
      padding: 10px 14px;
      border-radius: 10px;
      font-size: 14px;
      font-family: inherit;
      outline: none;
      transition: all 0.2s;
    }

    input:focus {
      border-color: var(--border-focus);
      box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.15);
    }

    button {
      margin-top: auto;
      width: 100%;
      color: #ffffff;
      border: none;
      padding: 12px 18px;
      border-radius: 10px;
      font-size: 14px;
      font-weight: 700;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      transition: all 0.2s;
      box-shadow: var(--shadow-sm);
    }

    button:hover {
      box-shadow: 0 6px 14px -3px rgba(0, 0, 0, 0.12);
      transform: translateY(-1px);
    }

    button:active {
      transform: translateY(0);
    }

    .btn-primary { background: var(--primary); }
    .btn-primary:hover { background: var(--primary-hover); }

    .btn-success { background: var(--success); }
    .btn-success:hover { background: var(--success-hover); }

    .btn-danger { background: var(--danger); }
    .btn-danger:hover { background: var(--danger-hover); }

    .btn-warning { background: var(--warning); }
    .btn-warning:hover { background: var(--warning-hover); }

    /* Console Card */
    .console-card {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      padding: 24px;
      box-shadow: var(--shadow-sm);
      margin-bottom: 28px;
    }

    .console-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 14px;
    }

    .console-title {
      font-size: 15px;
      font-weight: 700;
      color: var(--text-main);
    }

    .console-box {
      background: #f8fafc;
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 16px;
      font-family: 'JetBrains Mono', monospace;
      font-size: 13px;
      color: #1e293b;
      max-height: 220px;
      overflow-y: auto;
      white-space: pre-wrap;
      line-height: 1.6;
    }

    /* Audit Ledger Table */
    .table-card {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      padding: 24px;
      box-shadow: var(--shadow-sm);
      margin-bottom: 28px;
    }

    .table-card h2 {
      font-size: 17px;
      font-weight: 700;
      color: var(--text-main);
      margin-bottom: 6px;
    }

    .table-card p {
      font-size: 13px;
      color: var(--text-secondary);
      margin-bottom: 18px;
    }

    .table-wrapper {
      overflow-x: auto;
      border: 1px solid var(--border);
      border-radius: 10px;
    }

    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 13px;
      font-family: 'JetBrains Mono', monospace;
      background: #ffffff;
    }

    th {
      background: #f8fafc;
      color: var(--text-secondary);
      padding: 12px 16px;
      text-align: left;
      font-weight: 700;
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      border-bottom: 1.5px solid var(--border);
    }

    td {
      padding: 14px 16px;
      border-bottom: 1px solid #f1f5f9;
      color: var(--text-main);
    }

    tr:last-child td {
      border-bottom: none;
    }

    tr:hover td {
      background-color: #f8fafc;
    }

    .badge-type {
      display: inline-block;
      padding: 4px 10px;
      border-radius: 6px;
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.5px;
    }

    .badge-credit {
      background: var(--success-light);
      color: var(--success);
      border: 1px solid #a7f3d0;
    }

    .badge-debit {
      background: var(--danger-light);
      color: var(--danger);
      border: 1px solid #fecdd3;
    }

    /* Footer */
    .footer {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding-top: 18px;
      border-top: 1px solid var(--border);
      font-size: 13px;
      color: var(--text-secondary);
      flex-wrap: wrap;
      gap: 12px;
    }

    .footer a {
      color: var(--primary);
      text-decoration: none;
      font-weight: 600;
      margin-right: 18px;
    }

    .footer a:hover {
      text-decoration: underline;
    }

    @media (max-width: 900px) {
      .hero-banner {
        grid-template-columns: 1fr;
      }
    }
  </style>
</head>
<body>

  <div class="container">
    <!-- Navbar -->
    <header class="navbar">
      <div class="brand">
        <div class="brand-icon">🎮</div>
        <div class="brand-text">
          <h1>Game Wallet & Ledger Playground</h1>
          <p>Production High-Throughput Monetization Engine</p>
        </div>
      </div>
      <div class="status-pill">
        <span class="status-pulse"></span>
        <span id="healthLabel">All Systems Operational</span>
      </div>
    </header>

    <!-- Hero Balance Banner -->
    <section class="hero-banner">
      <div class="balance-card">
        <div class="balance-label">Player Wallet Balance</div>
        <div class="balance-amount" id="displayBalance">$0.00</div>
        <div class="balance-cents" id="displayUnits">0 minor units (cents)</div>
      </div>
      <div class="meta-details">
        <div class="meta-item">
          <div class="meta-item-label">Wallet ID</div>
          <div class="meta-item-value" id="metaWalletID">Initializing...</div>
        </div>
        <div class="meta-item">
          <div class="meta-item-label">Player Handle</div>
          <div class="meta-item-value" id="metaPlayerID">player_hero_99</div>
        </div>
        <div class="meta-item">
          <div class="meta-item-label">Currency</div>
          <div class="meta-item-value" id="metaCurrency">USD</div>
        </div>
        <div class="meta-item">
          <div class="meta-item-label">Optimistic Version</div>
          <div class="meta-item-value" id="metaVersion">v1</div>
        </div>
      </div>
    </section>

    <!-- Operational Cards Grid -->
    <div class="grid">
      <!-- 1. Create Wallet -->
      <div class="card">
        <div class="card-header">
          <div class="card-icon icon-blue">✦</div>
          <h2>1. Create Wallet</h2>
        </div>
        <div class="form-group">
          <label>Player ID</label>
          <input type="text" id="inputPlayerID" value="player_legend_77">
        </div>
        <div class="form-group">
          <label>Currency Code</label>
          <input type="text" id="inputCurrency" value="USD">
        </div>
        <button class="btn-primary" onclick="createWallet()">Create New Wallet</button>
      </div>

      <!-- 2. Deposit -->
      <div class="card">
        <div class="card-header">
          <div class="card-icon icon-green">↑</div>
          <h2>2. Deposit Funds</h2>
        </div>
        <div class="form-group">
          <label>Amount (in cents, 5000 = $50)</label>
          <input type="number" id="inputDepositAmount" value="5000">
        </div>
        <div class="form-group">
          <label>Description</label>
          <input type="text" id="inputDepositDesc" value="Credit card deposit $50.00">
        </div>
        <button class="btn-success" onclick="depositFunds()">Deposit Funds (+)</button>
      </div>

      <!-- 3. Purchase Item -->
      <div class="card">
        <div class="card-header">
          <div class="card-icon icon-rose">↓</div>
          <h2>3. Buy In-Game Item</h2>
        </div>
        <div class="form-group">
          <label>Amount (in cents, 1500 = $15)</label>
          <input type="number" id="inputPurchaseAmount" value="1500">
        </div>
        <div class="form-group">
          <label>Item SKU Code</label>
          <input type="text" id="inputPurchaseSKU" value="mythic_phoenix_blade">
        </div>
        <button class="btn-danger" onclick="purchaseItem()">Purchase Item (-)</button>
      </div>

      <!-- 4. Refund -->
      <div class="card">
        <div class="card-header">
          <div class="card-icon icon-amber">↺</div>
          <h2>4. Refund Purchase</h2>
        </div>
        <div class="form-group">
          <label>Original Transaction ID</label>
          <input type="text" id="inputRefundTxID" placeholder="Auto-filled from last purchase">
        </div>
        <div class="form-group">
          <label>Reason</label>
          <input type="text" id="inputRefundReason" value="Accidental purchase reversal">
        </div>
        <button class="btn-warning" onclick="refundTransaction()">Reverse Transaction (↺)</button>
      </div>
    </div>

    <!-- Live Console Output -->
    <div class="console-card">
      <div class="console-header">
        <div class="console-title">Live Server Response & Idempotency Monitor</div>
        <span style="font-size: 12px; color: var(--text-muted); font-weight: 600;">HTTP JSON Payloads</span>
      </div>
      <div class="console-box" id="consoleOutput">System ready. Click any action above to test...</div>
    </div>

    <!-- Immutable Double-Entry Ledger Table -->
    <div class="table-card">
      <h2>Immutable Double-Entry Ledger Entries (Audit Trail)</h2>
      <p>Each balance movement creates a permanent, tamper-evident audit record alongside the wallet balance update.</p>
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Entry ID</th>
              <th>Transaction ID</th>
              <th>Type</th>
              <th>Amount</th>
              <th>Balance Before</th>
              <th>Balance After</th>
              <th>Description</th>
              <th>Timestamp</th>
            </tr>
          </thead>
          <tbody id="ledgerTableBody">
            <tr><td colspan="8" style="text-align:center; padding: 24px; color: var(--text-muted);">No ledger entries yet. Initializing demo wallet...</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Footer Links -->
    <footer class="footer">
      <div>
        <a href="/health" target="_blank">Health Check Endpoint (/health)</a>
        <a href="/metrics" target="_blank">Prometheus Metrics (/metrics)</a>
      </div>
      <div>
        <span>Go 1.23+ • Gin • PostgreSQL pgx/v5 • Redis • RabbitMQ • Clean Architecture</span>
      </div>
    </footer>
  </div>

  <script>
    let activeWallet = null;

    function logConsole(title, data) {
      const el = document.getElementById('consoleOutput');
      const time = new Date().toLocaleTimeString();
      el.textContent = "=== " + title + " (" + time + ") ===\n" + JSON.stringify(data, null, 2);
    }

    function updateWalletView(w) {
      activeWallet = w;
      document.getElementById('metaWalletID').textContent = w.id;
      document.getElementById('metaPlayerID').textContent = w.player_id;
      document.getElementById('metaCurrency').textContent = w.currency;
      document.getElementById('metaVersion').textContent = 'v' + w.version;

      const formatted = (w.balance / 100).toLocaleString('en-US', { style: 'currency', currency: w.currency || 'USD' });
      document.getElementById('displayBalance').textContent = formatted;
      document.getElementById('displayUnits').textContent = w.balance + ' minor units / cents (' + w.currency + ')';
      
      loadLedger();
    }

    async function createWallet() {
      const player_id = document.getElementById('inputPlayerID').value;
      const currency = document.getElementById('inputCurrency').value;

      try {
        const res = await fetch('/api/v1/wallets', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ player_id, currency })
        });
        const json = await res.json();
        logConsole('CREATE WALLET RESPONSE', json);
        if (json.success && json.data) {
          updateWalletView(json.data);
        } else {
          alert('Error: ' + (json.error ? json.error.message : 'failed'));
        }
      } catch (err) {
        logConsole('ERROR', err.message);
      }
    }

    async function depositFunds() {
      if (!activeWallet) {
        alert('Please create a wallet first!');
        return;
      }
      const amount = parseInt(document.getElementById('inputDepositAmount').value);
      const description = document.getElementById('inputDepositDesc').value;
      const idempKey = 'dep-' + Date.now();

      try {
        const res = await fetch('/api/v1/wallets/' + activeWallet.id + '/deposit', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Idempotency-Key': idempKey
          },
          body: JSON.stringify({ amount, currency: activeWallet.currency, description })
        });
        const json = await res.json();
        logConsole('DEPOSIT RESPONSE (Idempotency-Key: ' + idempKey + ')', json);
        if (json.success && json.data) {
          updateWalletView(json.data.wallet);
        } else {
          alert('Deposit failed: ' + (json.error ? json.error.message : 'error'));
        }
      } catch (err) {
        logConsole('ERROR', err.message);
      }
    }

    async function purchaseItem() {
      if (!activeWallet) {
        alert('Please create a wallet first!');
        return;
      }
      const amount = parseInt(document.getElementById('inputPurchaseAmount').value);
      const item_sku = document.getElementById('inputPurchaseSKU').value;
      const idempKey = 'buy-' + Date.now();

      try {
        const res = await fetch('/api/v1/payments/purchase', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Idempotency-Key': idempKey
          },
          body: JSON.stringify({
            wallet_id: activeWallet.id,
            amount: amount,
            currency: activeWallet.currency,
            item_sku: item_sku,
            description: 'Item Purchase: ' + item_sku
          })
        });
        const json = await res.json();
        logConsole('PURCHASE RESPONSE (Idempotency-Key: ' + idempKey + ')', json);
        if (json.success && json.data) {
          document.getElementById('inputRefundTxID').value = json.data.transaction.id;
          updateWalletView(json.data.wallet);
        } else {
          alert('Purchase error: ' + (json.error ? json.error.message : 'error'));
        }
      } catch (err) {
        logConsole('ERROR', err.message);
      }
    }

    async function refundTransaction() {
      const origTxID = document.getElementById('inputRefundTxID').value.trim();
      const reason = document.getElementById('inputRefundReason').value;
      if (!origTxID) {
        alert('Please provide a purchase transaction ID to refund!');
        return;
      }
      const idempKey = 'ref-' + Date.now();

      try {
        const res = await fetch('/api/v1/payments/refund', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Idempotency-Key': idempKey
          },
          body: JSON.stringify({
            original_transaction_id: origTxID,
            reason: reason
          })
        });
        const json = await res.json();
        logConsole('REFUND RESPONSE (Idempotency-Key: ' + idempKey + ')', json);
        if (json.success && json.data) {
          updateWalletView(json.data.wallet);
        } else {
          alert('Refund error: ' + (json.error ? json.error.message : 'error'));
        }
      } catch (err) {
        logConsole('ERROR', err.message);
      }
    }

    async function loadLedger() {
      if (!activeWallet) return;
      try {
        const res = await fetch('/api/v1/wallets/' + activeWallet.id + '/ledger?limit=25');
        const json = await res.json();
        const tbody = document.getElementById('ledgerTableBody');
        tbody.innerHTML = '';

        if (!json.data || json.data.length === 0) {
          tbody.innerHTML = '<tr><td colspan="8" style="text-align:center; padding: 24px; color: var(--text-muted);">No entries yet in ledger</td></tr>';
          return;
        }

        json.data.forEach(item => {
          const tr = document.createElement('tr');
          const isCredit = item.entry_type === 'CREDIT';
          tr.innerHTML = 
            '<td title="' + item.id + '"><span style="color:var(--text-muted);">' + item.id.substring(0, 8) + '...</span></td>' +
            '<td title="' + item.transaction_id + '"><span style="color:var(--primary); font-weight:600;">' + item.transaction_id.substring(0, 8) + '...</span></td>' +
            '<td><span class="badge-type ' + (isCredit ? 'badge-credit' : 'badge-debit') + '">' + item.entry_type + '</span></td>' +
            '<td><strong style="color:' + (isCredit ? 'var(--success)' : 'var(--danger)') + ';">' + (isCredit ? '+' : '-') + (item.amount / 100).toFixed(2) + ' ' + item.currency + '</strong></td>' +
            '<td>' + (item.balance_before / 100).toFixed(2) + '</td>' +
            '<td><strong>' + (item.balance_after / 100).toFixed(2) + '</strong></td>' +
            '<td>' + (item.description || '-') + '</td>' +
            '<td style="color:var(--text-secondary);">' + new Date(item.created_at).toLocaleTimeString() + '</td>';
          tbody.appendChild(tr);
        });
      } catch (e) {
        console.error('Failed to load ledger', e);
      }
    }

    // Auto-create initial demo wallet and fund it on first launch
    window.addEventListener('load', async () => {
      await createWallet();
      // Auto initial deposit to make testing immediately fun
      setTimeout(async () => {
        if (activeWallet) {
          await depositFunds();
        }
      }, 300);
    });
  </script>
</body>
</html>
`

func (h *Handler) Dashboard(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, dashboardHTML)
}
