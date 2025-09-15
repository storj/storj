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
        await loginPage.selectManagedEnc(false);
        await loginPage.ensureSetupSuccess();
        await loginPage.finishSetup();

        await navigationMenu.navigateToAccountSettings();
        await accountSettingsPage.checkName(name);
        await accountSettingsPage.changeName(newName);
        await accountSettingsPage.checkName(newName);
    });

    test('Change password', async ({
        loginPage,
        signupPage,
        navigationMenu,
        accountSettingsPage,
        projectDashboardPage,
    }) => {
        const email = `${uuidv4()}@example.test`;
        const password = 'password';
        const newPassword = 'newPassword';
        const name = 'John Doe';

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
        await loginPage.selectManagedEnc(false);
        await loginPage.ensureSetupSuccess();
        await loginPage.finishSetup();

        await navigationMenu.navigateToAccountSettings();
        await accountSettingsPage.changePassword(password, newPassword);
        await navigationMenu.logout();
        await loginPage.loginByCreds(email, newPassword);
        await projectDashboardPage.verifyDashboardPage(name);
    });

    test('Edit session timeout', async ({
        loginPage,
        signupPage,
        navigationMenu,
        accountSettingsPage,
    }) => {
        const email = `${uuidv4()}@example.test`;
        const password = 'password';
        const name = 'John Doe';

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
        await loginPage.selectManagedEnc(false);
        await loginPage.ensureSetupSuccess();
        await loginPage.finishSetup();

        await navigationMenu.navigateToAccountSettings();
        await accountSettingsPage.verifySessionTimeout(' Currently set to 15 minutes. ');
        await accountSettingsPage.changeSessionTimeout('15 minutes', '1 hour');
        await accountSettingsPage.verifySessionTimeout(' Currently set to 1 hour. ');
    });
});
