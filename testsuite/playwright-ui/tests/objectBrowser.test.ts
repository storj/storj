// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';

test.describe('object browser + edge services', () => {
    test.beforeEach(async ({
        signupPage,
        loginPage,
        allProjectsPage,
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

        //await allProjectsPage.createProject(name);
        await navigationMenu.switchPassphrase(passphrase);
    });

    test('File download and upload', async ({
        objectBrowserPage,
        bucketsPage,
        navigationMenu,
    }) => {
        const fileName = 'test.txt';
        const bucketName = uuidv4();

        await navigationMenu.clickOnBuckets();
        await bucketsPage.createBucket(bucketName);
        await bucketsPage.openBucket(bucketName);
        await objectBrowserPage.waitLoading();
        await objectBrowserPage.uploadFile(fileName, 'text/plain');
        await objectBrowserPage.openObjectPreview(fileName, 'Text');

        // Checks if the link-sharing buttons work
        await objectBrowserPage.verifyObjectMapIsVisible();
        await objectBrowserPage.verifyShareLink();

        // Checks for successful download
        await objectBrowserPage.downloadFromPreview();
        await objectBrowserPage.closePreview(fileName);

        // Delete old file and upload new with the same file name
        await objectBrowserPage.deleteObjectByName(fileName, 'Text');
        await objectBrowserPage.uploadFile(fileName, 'text/csv');
        await objectBrowserPage.openObjectPreview(fileName, 'Text');
        await objectBrowserPage.verifyObjectMapIsVisible();
        await objectBrowserPage.verifyShareLink();
    });

    test('Folder creation and folder drag and drop upload', async ({
        bucketsPage,
        objectBrowserPage,
        navigationMenu,
    }) => {
        const bucketName = uuidv4();
        const fileName = 'test.txt';
        const folderName = 'test_folder';

        await navigationMenu.clickOnBuckets();
        await bucketsPage.createBucket(bucketName);
        await bucketsPage.openBucket(bucketName);

        // Create empty folder using New Folder Button
        await objectBrowserPage.createFolder(folderName);
        await objectBrowserPage.deleteObjectByName(folderName, 'Folder');

        // Folder creation with a file inside it
        await objectBrowserPage.uploadFolder(folderName, fileName, 'text/csv');
        await objectBrowserPage.deleteObjectByName(folderName, 'Folder');
    });

    test('Share bucket and bucket details page', async ({
        navigationMenu,
        bucketsPage,
        objectBrowserPage,
        common,
        page,
    }) => {
        const bucketName = uuidv4();
        const fileName = 'test1.jpeg';

        await navigationMenu.clickOnBuckets();
        await bucketsPage.createBucket(bucketName);
        await bucketsPage.openBucket(bucketName);
        await objectBrowserPage.waitLoading();
        await objectBrowserPage.uploadFile(fileName, 'image/jpeg');
        await objectBrowserPage.openObjectPreview(fileName, 'Image');

        // Checks the image preview of the tiny apple png file
        await objectBrowserPage.verifyImagePreviewIsVisible();
        await objectBrowserPage.closePreview(fileName);

        // Checks for Bucket Detail Header and correct bucket name
        await navigationMenu.clickOnBuckets();
        await bucketsPage.openBucketSettings();
        await bucketsPage.verifyBucketDetails(bucketName);
        await common.closeModal();

        // Check Bucket Share, see if copy button changed to copied
        await bucketsPage.openBucketSettings();
        await bucketsPage.verifyShareBucket();
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
});
