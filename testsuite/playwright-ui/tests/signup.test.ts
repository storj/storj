// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';

test.describe('Sign up personal/business accounts', () => {
    test.beforeEach(async ({
        loginPage,
        signupPage,
    }) => {
        const email = `${uuidv4()}@test.test`;
        const password = 'password';

        await signupPage.navigateToSignup();
        await signupPage.verifyHeader();
        await signupPage.verifySubheader();

        await signupPage.signupFirstStep(email, password);
        await signupPage.verifySuccessMessage();

        await signupPage.navigateToLogin();
        await loginPage.loginByCreds(email, password);
        await loginPage.verifySetupAccountFirstStep();
    });

    test('Signup Personal', async ({
        loginPage,
    }) => {
        const name = 'John Doe';
        await loginPage.choosePersonalAccSetup();
        await loginPage.fillPersonalSetupForm(name);
        await loginPage.selectFreeTrial();
        await loginPage.ensureSetupSuccess();
    });

    test('Signup Business', async ({
        loginPage,
    }) => {
        const firstName = 'John';
        const lastName = 'Doe';
        const companyName = 'Storj Labs';
        const jobRole = 'Software Developer';
        await loginPage.chooseBusinessAccSetup();
        await loginPage.fillBusinessSetupForm(firstName, lastName, companyName);
        await loginPage.selectFreeTrial();
        await loginPage.ensureSetupSuccess();
    });
});
