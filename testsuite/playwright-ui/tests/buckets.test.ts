// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';

test.describe('buckets', () => {
    test.beforeEach(async ({
        signupPage,
        loginPage,
        navigationMenu,
    }) => {
        const name = 'John Doe';
        const email = `${uuidv4()}@test.test`;
        const password = 'password';
        const passphrase = '1';

        await signupPage.navigateToSignup();
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

        await navigationMenu.switchPassphrase(passphrase);
    });

    test('Create and delete bucket', async ({
        navigationMenu,
        bucketsPage,
    }) => {
        const bucketName = 'testdelete';

        await navigationMenu.clickOnBuckets();
        await bucketsPage.createBucket(bucketName);
        await bucketsPage.openBucketSettings();
        await bucketsPage.verifyDeleteBucket(bucketName);
    });

    test('Create bucket with versioning', async ({
        navigationMenu,
        bucketsPage,
    }) => {
        const bucketName = 'test-versioning';

        await navigationMenu.clickOnBuckets();
        await bucketsPage.createBucketWithVersioning(bucketName);
        await bucketsPage.openBucketSettings();
        await bucketsPage.openBucketDetails();
        await bucketsPage.verifyEnabledStatus(2);
        await bucketsPage.closeBucketDetails();
        await bucketsPage.openBucketSettings();
        await bucketsPage.verifyDeleteBucket(bucketName);
    });

    test('Create bucket with object lock', async ({
        navigationMenu,
        bucketsPage,
    }) => {
        const bucketName = 'test-lock';

        await navigationMenu.clickOnBuckets();
        await bucketsPage.createBucketWithObjectLock(bucketName);
        await bucketsPage.openBucketSettings();
        await bucketsPage.openBucketDetails();
        await bucketsPage.verifyEnabledStatus(4); // 4 is expected here because "Enabled" appears twice in details dialog and twice in buckets table
        await bucketsPage.closeBucketDetails();
        await bucketsPage.openBucketSettings();
        await bucketsPage.verifyDeleteBucket(bucketName);
    });
});
