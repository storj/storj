// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';

test.describe('object browser + edge services', () => {
    test.beforeEach(async ({
        signupPageV2,
        loginPageV2,
        allProjectsPageV2,
        navigationMenuV2,
    }) => {
        const name = 'John Doe';
        const email = `${uuidv4()}@test.test`;
        const password = 'qazwsx';
        const passphrase = '1';

        await signupPageV2.navigateToSignup();
        await signupPageV2.signupFirstStep(email, password);
        await signupPageV2.verifySuccessMessage();
        await signupPageV2.navigateToLogin();

        await loginPageV2.loginByCreds(email, password);
        await loginPageV2.verifySetupAccountFirstStep();
        await loginPageV2.choosePersonalAccSetup();
        await loginPageV2.fillPersonalSetupForm(name);
        await loginPageV2.ensureSetupSuccess();
        await loginPageV2.finishSetup();

        await allProjectsPageV2.createProject(name);
        await navigationMenuV2.switchPassphrase(passphrase);
    });

    test('File download and upload', async ({
        objectBrowserPage,
        bucketsPageV2,
        navigationMenuV2,
    }) => {
        const fileName = 'test.txt';
        const bucketName = uuidv4();

        await navigationMenuV2.clickOnBuckets();
        await bucketsPageV2.createBucket(bucketName);
        await objectBrowserPage.waitLoading();
        await objectBrowserPage.uploadFile(fileName, 'text/plain');
        await objectBrowserPage.openObjectPreview(fileName, 'Text');

        // Checks for successful download
        await objectBrowserPage.downloadFromPreview();

        // Checks if the link-sharing buttons work
        await objectBrowserPage.verifyObjectMapIsVisible();
        await objectBrowserPage.verifyShareLink();
        await objectBrowserPage.closePreview(fileName);

        // Delete old file and upload new with the same file name
        await objectBrowserPage.deleteObjectByName(fileName, 'Text');
        await objectBrowserPage.uploadFile(fileName, 'text/csv');
        await objectBrowserPage.openObjectPreview(fileName, 'Text');
        await objectBrowserPage.verifyObjectMapIsVisible();
        await objectBrowserPage.verifyShareLink();
    });

    test('Folder creation and folder drag and drop upload', async ({
        bucketsPageV2,
        objectBrowserPage,
        navigationMenuV2,
    }) => {
        const bucketName = uuidv4();
        const fileName = 'test.txt';
        const folderName = 'test_folder';

        await navigationMenuV2.clickOnBuckets();
        await bucketsPageV2.createBucket(bucketName);

        // Create empty folder using New Folder Button
        await objectBrowserPage.createFolder(folderName);
        await objectBrowserPage.deleteObjectByName(folderName, 'Folder');

        // Folder creation with a file inside it
        await objectBrowserPage.uploadFolder(folderName, fileName, 'text/csv');
        await objectBrowserPage.deleteObjectByName(folderName, 'Folder');
    });

    test('Share bucket and bucket details page', async ({
        navigationMenuV2,
        bucketsPageV2,
        objectBrowserPage,
        commonV2,
        page,
    }) => {
        const bucketName = uuidv4();
        const fileName = 'test1.jpeg';

        await navigationMenuV2.clickOnBuckets();
        await bucketsPageV2.createBucket(bucketName);
        await objectBrowserPage.waitLoading();
        await objectBrowserPage.uploadFile(fileName, 'image/jpeg');
        await objectBrowserPage.openObjectPreview(fileName, 'Image');

        // Checks the image preview of the tiny apple png file
        await objectBrowserPage.verifyImagePreviewIsVisible();
        await objectBrowserPage.closePreview(fileName);

        // Checks for Bucket Detail Header and correct bucket name
        await page.goBack();
        await bucketsPageV2.openBucketSettings();
        await bucketsPageV2.verifyBucketDetails(bucketName);
        await commonV2.closeModal();

        // Check Bucket Share, see if copy button changed to copied
        await bucketsPageV2.openBucketSettings();
        await bucketsPageV2.verifyShareBucket();
    });

    test('Create and delete bucket', async ({
        navigationMenuV2,
        bucketsPageV2,
    }) => {
        const bucketName = 'testdelete';

        await navigationMenuV2.clickOnBuckets();
        await bucketsPageV2.createBucket(bucketName);
        await navigationMenuV2.clickOnBuckets();
        await bucketsPageV2.openBucketSettings();
        await bucketsPageV2.verifyDeleteBucket(bucketName);
    });
});
