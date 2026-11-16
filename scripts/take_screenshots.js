const puppeteer = require('puppeteer-core');
const path = require('path');
const fs = require('fs');

const chromePath = 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';
const outDir = path.join(__dirname, '..', 'docs', 'screenshots');

if (!fs.existsSync(outDir)) {
  fs.mkdirSync(outDir, { recursive: true });
}

async function run() {
  console.log('Launching Chrome via puppeteer-core...');
  const tmpProfile = path.join('C:\\Users\\hsnrz\\AppData\\Local\\Temp', 'puppeteer-screen-' + Date.now());

  const browser = await puppeteer.launch({
    executablePath: chromePath,
    headless: 'new',
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-gpu',
      '--no-first-run',
      '--no-default-browser-check',
      `--user-data-dir=${tmpProfile}`,
      '--window-size=1400,1100'
    ]
  });

  const page = await browser.newPage();
  page.on('dialog', async dialog => {
    console.log('Dialog dismissed:', dialog.message());
    await dialog.dismiss();
  });

  await page.setViewport({ width: 1400, height: 1100, deviceScaleFactor: 2 });

  console.log('Navigating to http://127.0.0.1:8080 ...');
  await page.goto('http://127.0.0.1:8080', { waitUntil: 'networkidle2', timeout: 15000 }).catch(() => {});

  // Wait 2s for initial auto-creation and auto-deposit
  await new Promise(r => setTimeout(r, 2000));

  // 1. Full Dashboard Overview Screenshot
  const overviewPath = path.join(outDir, 'dashboard-overview.png');
  await page.screenshot({ path: overviewPath, fullPage: false });
  console.log('Saved:', overviewPath);

  // 2. Ledger Audit Table Screenshot
  const tableEl = await page.$('.table-card');
  if (tableEl) {
    const tablePath = path.join(outDir, 'ledger-audit-trail.png');
    await tableEl.screenshot({ path: tablePath });
    console.log('Saved:', tablePath);
  }

  // 3. Operational Cards Screenshot
  const gridEl = await page.$('.grid');
  if (gridEl) {
    const gridPath = path.join(outDir, 'operations-panel.png');
    await gridEl.screenshot({ path: gridPath });
    console.log('Saved:', gridPath);
  }

  await browser.close();
  console.log('SUCCESS: All screenshots captured!');
}

run().catch(err => {
  console.error('Error taking screenshots:', err);
  process.exit(1);
});
