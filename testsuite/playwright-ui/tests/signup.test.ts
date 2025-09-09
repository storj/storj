// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';

test.describe('Sign up account', () => {
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

    test('Signup', async ({
        loginPage,
    }) => {
        const name = 'John Doe';
        const companyName = 'Storj Labs';
        await loginPage.fillSetupForm(name, companyName);
        await loginPage.selectFreeTrial();
        await loginPage.selectManagedEnc(false);
        await loginPage.ensureSetupSuccess();
    });
});
