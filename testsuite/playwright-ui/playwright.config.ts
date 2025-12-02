// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import * as os from 'os';
// import generateCustomLayoutSimpleMeta from './slackReporter';
import { defineConfig, ReporterDescription } from '@playwright/test';

// require('dotenv').config();

// Potentially interesting metadata to append into the test report – might help with debugging
const metadata: Record<string, string> = {
    cpu: os.arch(),
    memory: `${os.totalmem() / (1024 ** 2)} MB`,
    hostname: os.hostname(),
    system: os.type(),
    kernel: os.version(),
};

enum Reporter {
    HTML = 'html',
    List = 'list',
    CI = 'github'
}
/**
 * Customize reporters.
 * By default, we want to have a standard list reporting and a pretty HTML output.
 * In CI pipelines, we want to have an annotated report visible on the GitHub Actions page.
 */
const addReporter = (): ReporterDescription[] => {
    const defaultReporter: ReporterDescription[] = [
        [Reporter.List],
        [Reporter.HTML],
    ];

    if (isPipeline) {
        return defaultReporter.concat([[Reporter.CI]]);
    }

    return defaultReporter;
};

const isPipeline = !!process.env.CI;
const threshold = 0.95;

export default defineConfig({
    expect: {
        timeout: 4000, // Maximum time expect() should wait for the condition to be met.
        toMatchSnapshot: { threshold }, // Only require the screenshots to be the same within a certain threshold.
    },
    fullyParallel: true,
    outputDir: 'test-results/', // Folder for test artifacts such as screenshots, videos, traces, etc.
    projects: [
        {
            name: 'chromium-hd',
            use: {
                viewport: { width: 1280, height: 720 },
                browserName: 'chromium',
                headless: true,
                launchOptions: {
                    // args: ["--headless","--no-sandbox","--use-angle=gl"]
                    args: [
                        '--no-sandbox',
                        '--host-resolver-rules=MAP tenant1.localhost.test 127.0.0.1,MAP tenant2.localhost.test 127.0.0.1',
                    ],
                },
                permissions: ['clipboard-read', 'clipboard-write'],
            },
        },
        /*
        {
          name: 'chromium-fhd',
          use: {
            viewport: { width: 1920, height: 1080 },
            browserName: 'chromium',
          },
        },
        {
          name: 'firefox-hd',
          use: {
            viewport: { width: 1280, height: 720 },
            browserName: 'firefox',
          },
        },
        {
          name: 'webkit-hd',
          use: {
            viewport: { width: 1280, height: 720 },
            browserName: 'webkit',
          },
        },*/
    ],
    reporter: [
        [
            './node_modules/playwright-slack-report/dist/src/SlackReporter.js',
            {
                channels: ['#team-integrations-console-alerts'], // provide one or more Slack channels
                sendResults: 'always', // "always" , "on-failure", "off"
                showInThread: true,
            },
        ],
        ['dot'],
        ['html'],
    ],
    retries: process.env.CI ? 2 : 0, // Retry on CI only.
    testDir: './tests', // Directory where tests are located.
    timeout: 30 * 1000, // Maximum time one test can run for.
    use: {
        actionTimeout: 0, // Maximum time each action can take.
        // baseURL: 'http://127.0.0.1:10000',
        ignoreHTTPSErrors: true, // Suppress the errors relative to serving web data.
        trace: 'on-first-retry', // Collect trace when retrying the failed test.
        launchOptions: {
            slowMo: process.env.CI ? 0 : 0,
            headless: true,
        },
    },
    workers: process.env.CI ? 4 : undefined,
});
