// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';
import { createAndOnboardUser } from './common';

test.describe('Project members', () => {
    test.setTimeout(40000);

    const owner = `${uuidv4()}@example.com`;
    const member = `${uuidv4()}@example.com`;
    const password = 'password';
    const passphrase = '1';
    const name = 'test-name';

    test('Project invitation and member restrictions', async ({
        signupPage,
        loginPage,
        navigationMenu,
        bucketsPage,
        accessKeysPage,
        teamPage,
    }) => {
        await createAndOnboardUser({
            signupPage,
            loginPage,
            navigationMenu,
            email: owner,
            password,
            name: 'Inviter',
            companyName: 'Storj Labs',
            managedEnc: false,
        });
        await createAndOnboardUser({
            signupPage,
            loginPage,
            navigationMenu,
            email: member,
            password,
            name: 'Member',
            companyName: 'Storj Labs',
            managedEnc: false,
        });
        await loginPage.goToLogin();
        await loginPage.loginByCreds(owner, password);

        await navigationMenu.clickOnBuckets();
        await bucketsPage.createBucket(name);

        await navigationMenu.clickOnAccessKeys();
        await accessKeysPage.createAPIKey(name);

        await navigationMenu.clickOnTeam();
        await teamPage.waitForPage();
        await teamPage.confirmOwnerRoleChip();
        await teamPage.inviteMember(member);
        await teamPage.confirmInvitedRoleChip();
        await navigationMenu.logout();

        await loginPage.goToLogin();
        await loginPage.loginByCreds(member, password);

        await teamPage.joinProject();
        await navigationMenu.enterPassphrase(passphrase);

        await navigationMenu.clickOnTeam();
        await teamPage.confirmMemberRoleChip();

        await navigationMenu.clickOnBuckets();
        await bucketsPage.openBucketSettings();
        await bucketsPage.verifyCannotDeleteBucket();

        await navigationMenu.clickOnAccessKeys();
        await accessKeysPage.openAccessSettings();
        await accessKeysPage.verifyCannotDeleteAccess();
    });
});
