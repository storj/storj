// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';
import { createAndOnboardUser } from './common';

test.describe('Sign up account', () => {
    const email = `${uuidv4()}@test.test`;
    const password = 'password';

    test('Signup', async ({
        signupPage,
        loginPage,
        navigationMenu,
    }) => {
        await createAndOnboardUser({
            signupPage,
            loginPage,
            navigationMenu,
            email,
            password,
            name: 'John Doe',
            companyName: 'Storj Labs',
            managedEnc: false,
        });
    });
});
