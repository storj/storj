// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';

test.describe('Filebrowser + edge services', () => {
    test.beforeEach(async ({
        signupPage,
        loginPage,
        dashboardPage,
        allProjectsPage,
        bucketsPage,
        common,
    }, testInfo) => {
        console.log(`Running ${testInfo.title}`);

        const name = 'test';
        const email = `${uuidv4()}@storj.io`;
        const password = '123a123';
        const passphrase = 'qazwsx';

        await signupPage.navigateToSignup();
        await signupPage.signupApplicationPersonal(name, email, password);
        await signupPage.verifySuccessMessage();
        await loginPage.navigateToURL();
        await loginPage.loginByCreds(email, password);
        await allProjectsPage.createProject(name);
        await common.closeModal();
        await common.goToAllProjects();
        await allProjectsPage.clickOnProject(name);
        await bucketsPage.enterPassphrase(passphrase);
        await bucketsPage.clickContinueConfirmPassphrase();
        await dashboardPage.verifyWelcomeMessage();
    });

    // This test check file download, upload using drag and drop function and basic link-sharing features
    test('File download and upload', async ({
        bucketsPage,
        navigationMenu,
        common,
    }) => {
        const fileName = 'test.txt';
        const bucketName = uuidv4();

        await navigationMenu.clickOnBuckets();
        await common.waitLoading();
        await bucketsPage.enterNewBucketName(bucketName);
        await bucketsPage.clickContinueCreateBucket();
        await bucketsPage.dragAndDropFile(fileName, 'text/plain');

        // Checks for successful download
        await bucketsPage.downloadFromPreview(fileName);

        // Checks if the link-sharing buttons work
        await bucketsPage.verifyObjectMapIsVisible();
        await common.closeModal();
        await bucketsPage.clickShareButton();
        await common.waitLoading();
        await bucketsPage.clickCopyButtonShareBucketModal();
        await common.closeModal();
        await bucketsPage.closeFilePreview();

        // Delete old file and upload new with the same file name
        await bucketsPage.deleteFileByName(fileName);
        await bucketsPage.dragAndDropFile(fileName, 'text/csv');
        await bucketsPage.verifyObjectMapIsVisible();
        await common.closeModal();
        await bucketsPage.clickShareButton();
        await bucketsPage.clickCopyButtonShareBucketModal();
    });

    // This test check folder creation, upload using drag and drop function
    test('Folder creation and folder drag and drop upload', async ({
        bucketsPage,
        navigationMenu,
        common,
    }) => {
        const bucketName = uuidv4();
        const fileName = 'test.txt';
        const folderName = 'test_folder';

        await navigationMenu.clickOnBuckets();
        await common.waitLoading();
        await bucketsPage.enterNewBucketName(bucketName);
        await bucketsPage.clickContinueCreateBucket();

        // Create empty folder using New Folder Button
        await bucketsPage.createNewFolder(folderName);
        await bucketsPage.deleteFileByName(folderName);

        // DRAG AND DROP FOLDER creation with a file inside it for next instance of test
        await bucketsPage.dragAndDropFolder(folderName, fileName, 'text/csv');
        await bucketsPage.deleteFileByName(folderName);
    });

    test('Share bucket and bucket details page', async ({
        navigationMenu,
        bucketsPage,
        page,
        common,
    }) => {
        const bucketName = uuidv4();
        const fileName = 'test1.jpeg';

        await navigationMenu.clickOnBuckets();
        await common.waitLoading();
        await bucketsPage.enterNewBucketName(bucketName);
        await bucketsPage.clickContinueCreateBucket();
        await bucketsPage.dragAndDropFile(fileName, 'image/jpeg');

        // Checks the image preview of the tiny apple png file
        await bucketsPage.verifyImagePreviewIsVisible();
        await bucketsPage.closeFilePreview();

        // Checks for Bucket Detail Header and correct bucket name
        await bucketsPage.openBucketSettings();
        await bucketsPage.clickViewBucketDetails();
        await bucketsPage.verifyDetails(bucketName);
        await page.goBack();

        // Check Bucket Share, see if copy button changed to copied
        await bucketsPage.openBucketSettings();
        await bucketsPage.clickShareBucketButton();
        await bucketsPage.clickCopyButtonShareBucketModal();
    });

    test('Create and delete bucket', async ({
        navigationMenu,
        bucketsPage,
        common,
    }) => {
        const bucketName = 'testdelete';

        await navigationMenu.clickOnBuckets();
        await common.waitLoading();
        await bucketsPage.enterNewBucketName(bucketName);
        await bucketsPage.clickContinueCreateBucket();
        await navigationMenu.clickOnBuckets();
        await bucketsPage.openBucketDropdownByName(bucketName);
        await bucketsPage.clickDeleteBucketButton();
        await bucketsPage.enterBucketNameDeleteBucket(bucketName);
        await bucketsPage.clickConfirmDeleteButton();
        await common.waitLoading();
        await bucketsPage.verifyBucketNotVisible(bucketName);
    });
});
