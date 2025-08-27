// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';

test.describe('User settings', () => {
    test('Edit name', async ({
        loginPage,
        signupPage,
        navigationMenu,
        accountSettingsPage,
    }) => {
        const email = `${uuidv4()}@example.test`;
        const password = 'password';
        const name = 'John Doe';
        const newName = 'Jane Smith';

        await signupPage.navigateToSignup();
        await signupPage.verifyHeader();
        await signupPage.signupFirstStep(email, password);
        await signupPage.verifySuccessMessage();
        await signupPage.navigateToLogin();
        await loginPage.loginByCreds(email, password);
        await loginPage.verifySetupAccountFirstStep();
        await loginPage.choosePersonalAccSetup();
        await loginPage.fillPersonalSetupForm(name);
        await loginPage.selectFreeTrial();
        await loginPage.ensureSetupSuccess();
        await loginPage.finishSetup();

        await navigationMenu.navigateToAccountSettings();
        await accountSettingsPage.checkName(name);
        await accountSettingsPage.changeName(newName);
        await accountSettingsPage.checkName(newName);
    });
});
