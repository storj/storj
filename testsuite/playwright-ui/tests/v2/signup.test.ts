// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';

test.describe('Sign up personal/business accounts', () => {
    test.beforeEach(async ({
        loginPageV2,
        signupPageV2,
    }) => {
        const email = `${uuidv4()}@test.test`;
        const password = 'qazwsx';

        await signupPageV2.navigateToSignup();
        await signupPageV2.verifyHeader();
        await signupPageV2.verifySubheader();

        await signupPageV2.signupFirstStep(email, password);
        await signupPageV2.verifySuccessMessage();

        await signupPageV2.navigateToLogin();
        await loginPageV2.loginByCreds(email, password);
        await loginPageV2.verifySetupAccountFirstStep();
    });

    test('Signup Personal', async ({
        loginPageV2,
    }) => {
        const name = 'John Doe';
        await loginPageV2.choosePersonalAccSetup();
        await loginPageV2.fillPersonalSetupForm(name);
        await loginPageV2.ensureSetupSuccess();
    });

    test('Signup Business', async ({
        loginPageV2,
    }) => {
        const firstName = 'John';
        const lastName = 'Doe';
        const companyName = 'Storj Labs';
        const jobRole = 'Awesome Developer';
        await loginPageV2.chooseBusinessAccSetup();
        await loginPageV2.fillBusinessSetupForm(firstName, lastName, companyName, jobRole);
        await loginPageV2.ensureSetupSuccess();
    });
});
