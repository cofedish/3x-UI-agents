const { defineConfig } = require('@playwright/test');

const baseURL = process.env.PANEL_URL || 'http://panel:2053/lab';
const outputDir = process.env.ARTIFACTS_DIR
  ? `${process.env.ARTIFACTS_DIR}/playwright`
  : 'playwright-output';

module.exports = defineConfig({
  testDir: './tests',
  timeout: 120000,
  fullyParallel: false,
  workers: 1,
  expect: {
    timeout: 20000,
  },
  reporter: [
    ['list'],
    ['html', { outputFolder: outputDir, open: 'never' }],
  ],
  use: {
    baseURL,
    headless: true,
    acceptDownloads: true,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  outputDir: `${outputDir}/output`,
});
