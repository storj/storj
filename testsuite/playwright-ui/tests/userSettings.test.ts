// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';
import { createAndOnboardUser } from '@config/tests/common';

test.describe('User settings', () => {
    const email = `${uuidv4()}@example.com`;
    const name = 'John Doe';
    const companyName = 'Storj Labs';
    const password = 'password';
    let userCreated = false;

    test.beforeEach(async ({
        signupPage,
        loginPage,
        navigationMenu,
    }) => {
        const passphrase = '1';

        if (!userCreated) {
            await createAndOnboardUser({
                signupPage,
                loginPage,
                navigationMenu,
                email,
                password,
                name,
                companyName,
                managedEnc: false,
            });
            userCreated = true;
        }

        await loginPage.goToLogin();
        await loginPage.loginByCreds(email, password);
        await navigationMenu.switchPassphrase(passphrase);
    });

    test('Edit name', async ({
        navigationMenu,
        accountSettingsPage,
    }) => {
        const newName = 'Jane Smith';

        await navigationMenu.navigateToAccountSettings();
        await accountSettingsPage.checkName(name);
        await accountSettingsPage.changeName(newName);
        await accountSettingsPage.checkName(newName);
    });

    test('Edit session timeout', async ({
        navigationMenu,
        accountSettingsPage,
    }) => {
        await navigationMenu.navigateToAccountSettings();
        await accountSettingsPage.verifySessionTimeout(' Currently set to 15 minutes. ');
        await accountSettingsPage.changeSessionTimeout('15 minutes', '1 hour');
        await accountSettingsPage.verifySessionTimeout(' Currently set to 1 hour. ');
    });

    test('Change password', async ({
        loginPage,
        navigationMenu,
        accountSettingsPage,
        projectDashboardPage,
    }) => {
        const newPassword = 'newPassword';

        await navigationMenu.navigateToAccountSettings();
        await accountSettingsPage.changePassword(password, newPassword);
        await navigationMenu.logout();
        await loginPage.loginByCreds(email, newPassword);
        await projectDashboardPage.verifyDashboardPage();
    });
});
