// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

// @ts-ignore
import os from 'node:os';
import {GenerateCustomLayoutSimpleMeta} from './slackReporter';
import { PlaywrightTestConfig, devices} from '@playwright/test';

// require('dotenv').config();

// Potentially interesting metadata to append into the test report – might help with debugging
const metadata: Record<string, string> = {
  cpu: os.arch(),
  memory: `${os.totalmem() / (1024 ** 2)} MB`,
  hostname: os.hostname(),
  system: os.type(),
  kernel: os.version(),
};

// Match pixel comparison at least 95 % to avoid flaky tests but ensure enough confidence
const threshold = 0.95;

const config: PlaywrightTestConfig = {
    testDir: './tests',                                   /* directory where tests are located.  */
    timeout: 30 * 1000,                                 /* Maximum time one test can run for.  */

    expect: {
        timeout: 4000,                                    /* Maximum time expect() should wait for the condition to be met. */
        toMatchSnapshot: {threshold},                   /* only require the screenshots to be the same within a certain threshold */
    },

    fullyParallel: false,                                /* Run tests in files in parallel */

    retries: process.env.CI ? 1 : 0,                    /* Retry on CI only */

    workers: process.env.CI ? 1 : undefined,                    /* Opt out of parallel tests on CI. */

    reporter: [
        [
            "./node_modules/playwright-slack-report/dist/src/SlackReporter.js",
            {
                channels: ["#team-integrations-console-alerts", "team-qa-github"], // provide one or more Slack channels
                sendResults: "always", // "always" , "on-failure", "off"
            },
        ],
        ["allure-playwright"]
  ],
    use: {                                              /* Shared settings for all the projects below. */
        actionTimeout: 0,                                 /* Maximum time each action can take. */
        // baseURL: 'http://nightly.storj.rodeo/',     /* Base URL to use in actions like `await page.goto('/')`. */
         baseURL: 'http://localhost:10000',
        // headless: process.env.CI ? false : true,       /* Starts the UI tests in headed mode, so we can watch execution in development */
        ignoreHTTPSErrors: true,                          /* suppress the errors relative to serving web data   */
        trace: 'on-first-retry',                          /* Collect trace when retrying the failed test. */
        screenshot: 'only-on-failure',

        launchOptions: {
            slowMo: process.env.CI ? 0 : 0,
        },
    },
    /* Configure projects for major browsers */
    projects: [
        {
            name: 'chromium',
            use: {
                ...devices['Desktop Chrome'],
            },
        },

        {
            name: 'firefox',
            use: {
                ...devices['Desktop Firefox'],
            },
        },

        {
            name: 'safari',
            use: {
                ...devices['Desktop Safari'],
            },
        },
        {
            name: 'Edge',
            ...devices['Desktop Edge'],
        },
        /* Test against mobile viewports. */
        {
            name: 'Android',
            use: {
                ...devices['Pixel 5'],
            },
        },
        {
            name: 'iPhone(13)',
            use: {
                ...devices['iPhone 13'],
            },
        },
    ],
    /* Folder for test artifacts such as screenshots, videos, traces, etc. */
    outputDir: 'test-results/',
};

export default config;
