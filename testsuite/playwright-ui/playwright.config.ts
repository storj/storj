// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

// @ts-ignore
import os from 'node:os';
import generateCustomLayoutSimpleMeta from './slackReporter';
import {PlaywrightTestConfig, devices, ReporterDescription} from '@playwright/test';

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
}

const isPipeline = !!process.env.CI;
const threshold = 0.95;

const config: PlaywrightTestConfig = {
     expect: {
         timeout: 4000,                                    /* Maximum time expect() should wait for the condition to be met. */
         toMatchSnapshot: {threshold},                   /* only require the screenshots to be the same within a certain threshold */
     },                                   /* directory where tests are located.  */
     fullyParallel: false,                                 /* Maximum time one test can run for.  */
     outputDir: 'test-results/',
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
                use: {
                 ...devices['Desktop Edge'],
                },
         },   /* Test against mobile viewports. */

         {
            name: 'Android',
                use: {
                    ...devices['Pixel 5'],
                },
         },
        ],                                /* Run tests in files in parallel */

        reporter: [
            [
                "./node_modules/playwright-slack-report/dist/src/SlackReporter.js", 
                {
                    channels: ["#team-integrations-console-alerts"], // provide one or more Slack channels
                    sendResults: "always", // "always" , "on-failure", "off"
                    showInThread: true,
                },
            ],
            ["dot"],
        ],                    /* Retry on CI only */

        retries: process.env.CI ? 1 : 0,                    /* Opt out of parallel tests on CI. */
        testDir: './tests',
        timeout: 10 * 1000,
        use: {                                              /* Shared settings for all the projects below. */
            actionTimeout: 0,                                 /* Maximum time each action can take. */
            baseURL: 'http://127.0.0.1:10000',
            ignoreHTTPSErrors: true,                          /* suppress the errors relative to serving web data   */
            trace: 'on-first-retry',                          /* Collect trace when retrying the failed test. */
            launchOptions: {
            slowMo: process.env.CI ? 0 : 0,
            headless: true,
            },
        },
        /* Folder for test artifacts such as screenshots, videos, traces, etc. */
        workers: process.env.CI ? 1 : undefined,
    };


export default config;
