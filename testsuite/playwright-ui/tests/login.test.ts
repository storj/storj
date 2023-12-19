// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';

test.describe('Login Test', () => {
    test('Goto URL', async ({ loginPage }, testInfo) => {
        console.log(`Running ${testInfo.title}`);

        await loginPage.navigateToURL();
    });
    test('Login', async ({ loginPage }, testInfo) => {
        console.log(`Running ${testInfo.title}`);

        await loginPage.navigateToURL();
        await loginPage.loginToApplication();
    });
});
