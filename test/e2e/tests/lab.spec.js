const { test, expect } = require('@playwright/test');

const panelUrl = process.env.PANEL_URL || 'http://panel:2053/lab';
const username = process.env.PANEL_USERNAME || 'admin';
const password = process.env.PANEL_PASSWORD || 'admin123';
const origin = new URL(panelUrl).origin;

async function setEnglishCookie(page) {
  await page.context().addCookies([
    {
      name: 'lang',
      value: 'en-US',
      url: origin,
    },
  ]);
}

function createMonitor(page) {
  const state = {
    consoleErrors: [],
    pageErrors: [],
    responseErrors: [],
  };

  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      state.consoleErrors.push(msg.text());
    }
  });

  page.on('pageerror', (err) => {
    state.pageErrors.push(err.message);
  });

  page.on('response', (resp) => {
    const url = resp.url();
    if (!url.startsWith(origin)) {
      return;
    }
    if (!url.includes('/panel/')) {
      return;
    }
    if (resp.status() >= 500) {
      state.responseErrors.push(`${resp.status()} ${url}`);
    }
  });

  return {
    reset() {
      state.consoleErrors.length = 0;
      state.pageErrors.length = 0;
      state.responseErrors.length = 0;
    },
    state,
  };
}

async function login(page) {
  await setEnglishCookie(page);
  await page.goto('/login');
  const usernameInput = page.locator('input[name="username"]');
  if (await usernameInput.isVisible()) {
    await usernameInput.fill(username);
    await page.locator('input[name="password"]').fill(password);
    await page.locator('button[type="submit"]').click();
  }
  await page.waitForURL(/\/panel\/?$/);
}

async function waitForPageReady(page) {
  await page.waitForSelector('#app');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(1000);
}

async function assertNoMissingTranslations(page) {
  const missing = await page.evaluate(() => {
    const text = document.body.innerText || '';
    const patterns = [
      /\bpages\.[A-Za-z0-9_.-]+\b/g,
      /\bmenu\.[A-Za-z0-9_.-]+\b/g,
      /\bsecAlert[A-Za-z0-9_.-]*\b/g,
      /\bselectServer\b/g,
      /\ballServers\b/g,
      /\blocalServer\b/g,
      /\bdefaultLabel\b/g,
    ];
    const hits = new Set();
    for (const re of patterns) {
      let match = re.exec(text);
      while (match) {
        hits.add(match[0]);
        match = re.exec(text);
      }
    }
    return Array.from(hits);
  });
  expect(missing, `missing i18n keys: ${missing.join(', ')}`).toEqual([]);
}

async function assertNoBrokenIcons(page) {
  const broken = await page.evaluate(() => {
    return Array.from(document.querySelectorAll('.anticon'))
      .filter((icon) => {
        if (icon.querySelector('svg')) {
          return false;
        }
        if (icon.querySelector('img')) {
          return false;
        }
        return icon.textContent.trim() === '';
      })
      .map((icon) => icon.className);
  });
  expect(broken, `broken icons: ${broken.join(', ')}`).toEqual([]);
}

async function fillByTestId(page, testId, value) {
  const target = page.locator(`[data-testid="${testId}"]`);
  await expect(target).toBeVisible();
  const input = target.locator('input');
  if (await input.count()) {
    await input.fill(value);
    return;
  }
  await target.fill(value);
}

async function confirmPopconfirm(page) {
  const pop = page.locator('.ant-popconfirm');
  await expect(pop).toBeVisible();
  await pop.locator('button.ant-btn-primary').click();
}

function assertApprox(actual, expected, label, pct = 0.1, abs = 1) {
  if (typeof actual !== 'number' || typeof expected !== 'number') {
    return;
  }
  const diff = Math.abs(actual - expected);
  const limit = Math.max(abs, Math.abs(expected) * pct);
  expect(
    diff,
    `${label} diff too high: actual=${actual} expected=${expected} diff=${diff} limit=${limit}`
  ).toBeLessThanOrEqual(limit);
}

test.describe.serial('lab e2e', () => {
  test('route crawler', async ({ page }) => {
    const monitor = createMonitor(page);
    await login(page);

    const routes = [
      '/panel/',
      '/panel/inbounds',
      '/panel/servers',
      '/panel/settings',
      '/panel/xray',
    ];

    for (const route of routes) {
      monitor.reset();
      const response = await page.goto(route);
      expect(response, `${route} response`).not.toBeNull();
      expect(response.status(), `${route} status`).toBeLessThan(400);
      await waitForPageReady(page);
      await assertNoMissingTranslations(page);
      await assertNoBrokenIcons(page);
      expect(monitor.state.consoleErrors, 'console errors').toEqual([]);
      expect(monitor.state.pageErrors, 'page errors').toEqual([]);
      expect(monitor.state.responseErrors, '500 responses').toEqual([]);
    }
  });

  test('servers online and dashboard metrics align', async ({ page }) => {
    await login(page);
    await page.goto('/panel/servers');
    await waitForPageReady(page);

    const mtlsRow = page.locator('tr', { hasText: 'https://agent-mtls:2054' }).first();
    const jwtRow = page.locator('tr', { hasText: 'https://agent-jwt:2054' }).first();
    await expect(mtlsRow).toBeVisible();
    await expect(jwtRow).toBeVisible();
    await expect(mtlsRow.locator('.ant-badge-status-success')).toBeVisible();
    await expect(jwtRow.locator('.ant-badge-status-success')).toBeVisible();

    const serverList = await page.request.get('/panel/api/servers');
    const serversJson = await serverList.json();
    const servers = serversJson.obj?.servers || [];
    const serverIds = Array.from(new Set([1, ...servers.map((s) => s.id)])).filter(
      (id) => Number.isFinite(id) && id > 0
    );

    for (const serverId of serverIds) {
      await page.evaluate((id) => localStorage.setItem('selectedServerId', String(id)), serverId);
      await page.goto('/panel/');
      await page.waitForFunction(() => {
        const appRef = typeof app !== 'undefined' ? app : window.app;
        return appRef && appRef.status && appRef.status.cpu;
      });
      const uiStatus = await page.evaluate(() => {
        const appRef = typeof app !== 'undefined' ? app : window.app;
        return {
          cpu: appRef.status.cpu.current,
          memCurrent: appRef.status.mem.current,
          memTotal: appRef.status.mem.total,
          diskCurrent: appRef.status.disk.current,
          diskTotal: appRef.status.disk.total,
          swapCurrent: appRef.status.swap.current,
          swapTotal: appRef.status.swap.total,
          netUp: appRef.status.netIO.up,
          netDown: appRef.status.netIO.down,
          trafficSent: appRef.status.netTraffic.sent,
          trafficRecv: appRef.status.netTraffic.recv,
          uptime: appRef.status.uptime,
          appStats: appRef.status.appStats,
          xray: appRef.status.xray,
        };
      });

      const apiResp = await page.request.get(`/panel/api/server/status?server_id=${serverId}`);
      const apiJson = await apiResp.json();
      const apiStatus = apiJson.obj;

      assertApprox(uiStatus.cpu, apiStatus.cpu, `cpu:${serverId}`, 0.2, 2);
      assertApprox(uiStatus.memCurrent, apiStatus.mem.current, `memCurrent:${serverId}`, 0.1, 1024 * 1024);
      assertApprox(uiStatus.memTotal, apiStatus.mem.total, `memTotal:${serverId}`, 0.05, 1024 * 1024);
      assertApprox(uiStatus.diskCurrent, apiStatus.disk.current, `diskCurrent:${serverId}`, 0.1, 5 * 1024 * 1024);
      assertApprox(uiStatus.diskTotal, apiStatus.disk.total, `diskTotal:${serverId}`, 0.05, 5 * 1024 * 1024);
      assertApprox(uiStatus.swapCurrent, apiStatus.swap.current, `swapCurrent:${serverId}`, 0.1, 1024 * 1024);
      assertApprox(uiStatus.swapTotal, apiStatus.swap.total, `swapTotal:${serverId}`, 0.05, 1024 * 1024);
      assertApprox(uiStatus.netUp, apiStatus.netIO.up, `netUp:${serverId}`, 0.2, 1024);
      assertApprox(uiStatus.netDown, apiStatus.netIO.down, `netDown:${serverId}`, 0.2, 1024);
      assertApprox(uiStatus.trafficSent, apiStatus.netTraffic.sent, `trafficSent:${serverId}`, 0.2, 1024);
      assertApprox(uiStatus.trafficRecv, apiStatus.netTraffic.recv, `trafficRecv:${serverId}`, 0.2, 1024);
      assertApprox(uiStatus.uptime, apiStatus.uptime, `uptime:${serverId}`, 0.2, 5);
      expect(uiStatus.xray.state).toBe(apiStatus.xray.state);
    }
  });

  test('inbounds and clients CRUD', async ({ page }) => {
    await login(page);
    await page.goto('/panel/inbounds');
    await waitForPageReady(page);

    const remark = `lab-inbound-${Date.now()}`;
    const updatedRemark = `${remark}-updated`;
    const port = String(18000 + Math.floor(Math.random() * 2000));

    await page.locator('[data-testid="inbounds-add"]').click();
    await expect(page.locator('#inbound-modal')).toBeVisible();
    await fillByTestId(page, 'inbound-remark', remark);
    await fillByTestId(page, 'inbound-port', port);
    await page.locator('#inbound-modal').getByRole('button', { name: /create|ok|sure/i }).click();

    const row = page.locator('tr', { hasText: remark }).first();
    await expect(row).toBeVisible();

    await row.locator('.anticon-more').click();
    await page.getByRole('menuitem', { name: /edit/i }).click();
    await expect(page.locator('#inbound-modal')).toBeVisible();
    await fillByTestId(page, 'inbound-remark', updatedRemark);
    await page.locator('#inbound-modal').getByRole('button', { name: /update|ok|sure/i }).click();

    const updatedRow = page.locator('tr', { hasText: updatedRemark }).first();
    await expect(updatedRow).toBeVisible();

    const enableSwitch = updatedRow.locator('.ant-switch').first();
    const wasChecked = (await enableSwitch.getAttribute('class'))?.includes('ant-switch-checked');
    await enableSwitch.click();
    await page.waitForTimeout(500);
    const isChecked = (await enableSwitch.getAttribute('class'))?.includes('ant-switch-checked');
    expect(isChecked).toBe(!wasChecked);
    await enableSwitch.click();

    await updatedRow.locator('.anticon-more').click();
    await page.getByRole('menuitem', { name: /add client/i }).click();
    await expect(page.locator('#client-modal')).toBeVisible();

    const clientEmail = `lab-client-${Date.now()}@local`;
    const clientComment = 'lab-client';
    await fillByTestId(page, 'client-email', clientEmail);
    await fillByTestId(page, 'client-comment', clientComment);
    await page.locator('#client-modal').getByRole('button', { name: /add|ok|sure/i }).click();

    await updatedRow.locator('.ant-table-row-expand-icon').click();
    const clientRow = page.locator('tr', { hasText: clientEmail }).first();
    await expect(clientRow).toBeVisible();

    await clientRow.locator('.anticon-edit').click();
    await expect(page.locator('#client-modal')).toBeVisible();
    await fillByTestId(page, 'client-comment', `${clientComment}-updated`);
    await page.locator('#client-modal').getByRole('button', { name: /update|ok|sure/i }).click();

    await clientRow.locator('.anticon-delete').click();
    await confirmPopconfirm(page);
    await expect(page.locator('span.client-email', { hasText: clientEmail })).toHaveCount(0);

    await updatedRow.locator('.anticon-more').click();
    await page.getByRole('menuitem', { name: /^delete$/i }).click();
    await confirmPopconfirm(page);
    await expect(page.locator('tr', { hasText: updatedRemark })).toHaveCount(0);
  });

  test('xray controls and logs', async ({ page }) => {
    await login(page);
    await page.goto('/panel/');
    await waitForPageReady(page);

    const restartPromise = page.waitForResponse((resp) =>
      resp.url().includes('/panel/api/server/restartXrayService') && resp.status() === 200
    );
    await page.locator('[data-testid="dashboard-xray-restart"]').click();
    await restartPromise;

    await page.locator('[data-testid="dashboard-logs"]').click();
    const logModal = page.locator('#log-modal');
    await expect(logModal).toBeVisible();
    await expect(logModal.locator('.log-container')).not.toHaveText(/No Record/i);
  });

  test('remote xray logs use selected server', async ({ page }) => {
    await login(page);

    const serverList = await page.request.get('/panel/api/servers');
    const serversJson = await serverList.json();
    const servers = serversJson.obj?.servers || [];
    const remote = servers.find((server) => server.id && server.id > 1);
    if (!remote) {
      test.skip(true, 'no remote servers available');
    }

    await page.evaluate((id) => localStorage.setItem('selectedServerId', String(id)), remote.id);
    await page.goto('/panel/');
    await waitForPageReady(page);

    const responsePromise = page.waitForResponse((resp) => {
      return (
        resp.url().includes('/panel/api/server/xraylogs/') &&
        resp.url().includes(`server_id=${remote.id}`) &&
        resp.status() === 200
      );
    });
    await page.evaluate(() => {
      const appRef = typeof app !== 'undefined' ? app : window.app;
      if (appRef && appRef.openXrayLogs) {
        appRef.openXrayLogs();
      }
    });
    const response = await responsePromise;
    const payload = await response.json();
    expect(payload.success).toBe(true);
    expect(Array.isArray(payload.obj)).toBeTruthy();
  });

  test('settings toggle', async ({ page }) => {
    await login(page);
    await page.goto('/panel/settings');
    await waitForPageReady(page);

    const toggle = page.locator('[data-testid="settings-external-traffic-enable"]');
    const before = await toggle.getAttribute('class');
    await toggle.click();
    await page.locator('[data-testid="settings-save"]').click();
    await page.waitForResponse((resp) =>
      resp.url().includes('/panel/api/setting/update') && resp.status() === 200
    );

    await toggle.click();
    await page.locator('[data-testid="settings-save"]').click();
    await page.waitForResponse((resp) =>
      resp.url().includes('/panel/api/setting/update') && resp.status() === 200
    );
    const after = await toggle.getAttribute('class');
    expect(after).toBe(before);
  });

  test('backup export/import', async ({ page }, testInfo) => {
    await login(page);
    await page.goto('/panel/');
    await waitForPageReady(page);

    await page.locator('[data-testid="dashboard-backup"]').click();
    const backupModal = page.locator('#backup-modal');
    await expect(backupModal).toBeVisible();

    const downloadPromise = page.waitForEvent('download');
    await page.locator('[data-testid="backup-export"]').click();
    const download = await downloadPromise;
    const dbPath = testInfo.outputPath('lab-backup.db');
    await download.saveAs(dbPath);

    const chooserPromise = page.waitForEvent('filechooser');
    await page.locator('[data-testid="backup-import"]').click();
    const chooser = await chooserPromise;
    await chooser.setFiles(dbPath);

    await page.waitForLoadState('domcontentloaded', { timeout: 120000 });
    const loginField = page.locator('input[name="username"]');
    if (await loginField.count()) {
      if (await loginField.isVisible()) {
        await login(page);
      }
    }
    await page.goto('/panel/');
    await waitForPageReady(page);
  });
});
