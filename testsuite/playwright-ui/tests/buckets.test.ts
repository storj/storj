// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';
import { BucketsPageObjects } from '@objects/BucketsPageObjects';

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
        await loginPage.selectManagedEnc(false);
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

    test('Share bucket and bucket details page', async ({
        navigationMenu,
        bucketsPage,
        objectBrowserPage,
    }) => {
        const bucketName = uuidv4();
        const fileName = 'test1.jpeg';

        await navigationMenu.clickOnBuckets();
        await bucketsPage.createBucket(bucketName);
        await bucketsPage.openBucket(bucketName);
        await objectBrowserPage.waitForPage();
        await objectBrowserPage.uploadFile(fileName, 'image/jpeg');
        await objectBrowserPage.clickItem(fileName);

        // Checks the image preview of the tiny apple png file
        await objectBrowserPage.verifyImagePreviewIsVisible();
        await objectBrowserPage.closePreview();

        // Checks for Bucket Detail Header and correct bucket name
        await navigationMenu.clickOnBuckets();
        await bucketsPage.openBucketSettings();
        await bucketsPage.verifyBucketDetails(bucketName);

        // Check Bucket Share, see if copy button changed to copied
        await bucketsPage.openBucketSettings();
        await bucketsPage.verifyShareBucket();
    });

    test('Create bucket with placement', async ({
        navigationMenu,
        bucketsPage,
    }) => {
        const bucketName0 = 'test-placement-0';
        const bucketName1 = 'test-placement-1';

        await navigationMenu.clickOnBuckets();
        await bucketsPage.createBucketWithPlacement(bucketName0, BucketsPageObjects.NEW_BUCKET_GLOBAL_PLACEMENT_BUTTON_XPATH, 'Global');
        await bucketsPage.openBucketSettings();
        await bucketsPage.openBucketDetails();
        await bucketsPage.verifyLocation('global');
        await bucketsPage.closeBucketDetails();
        await bucketsPage.openBucketSettings();
        await bucketsPage.verifyDeleteBucket(bucketName0);
        await bucketsPage.createBucketWithPlacement(bucketName1, BucketsPageObjects.NEW_BUCKET_SELECT_PLACEMENT_BUTTON_XPATH, 'Storj Select');
        await bucketsPage.openBucketSettings();
        await bucketsPage.openBucketDetails();
        await bucketsPage.verifyLocation('us-select-1');
        await bucketsPage.closeBucketDetails();
    });
});
