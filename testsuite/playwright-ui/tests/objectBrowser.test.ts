// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';
import { join } from 'path';

test.describe('object browser + edge services', () => {
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
        await objectBrowserPage.waitForPage();
        await objectBrowserPage.uploadFile(fileName, 'text/plain');
        await objectBrowserPage.clickItem(fileName);

        // Checks if the link-sharing buttons work
        await objectBrowserPage.verifyObjectMapIsVisible();
        await objectBrowserPage.verifyShareObjectLink();

        // Checks for successful download
        await objectBrowserPage.downloadFromPreview();
        await objectBrowserPage.closePreview();

        // Delete old file and upload new with the same file name
        await objectBrowserPage.deleteItemByName(fileName);
        await objectBrowserPage.uploadFile(fileName, 'text/csv');
        await objectBrowserPage.clickItem(fileName);
        await objectBrowserPage.verifyObjectMapIsVisible();
        await objectBrowserPage.verifyShareObjectLink();
    });

    test('Bulk file deletion', async ({
        objectBrowserPage,
        bucketsPage,
        navigationMenu,
    }) => {
        const bucketName = uuidv4();
        const folderName = 'testdata';

        await navigationMenu.clickOnBuckets();
        await bucketsPage.createBucket(bucketName);
        await bucketsPage.openBucket(bucketName);
        await objectBrowserPage.waitForPage();

        for (const fileName of ['a.txt', 'b.txt', 'c.txt', 'd.txt']) {
            await objectBrowserPage.uploadFile(fileName, 'text/plain');
        }

        // Ensure that bulk deletion of individually-selected objects succeeds.
        await objectBrowserPage.selectItem('a.txt');
        await objectBrowserPage.selectItem('b.txt');
        await objectBrowserPage.deleteSelectedItems();
        await objectBrowserPage.expectItems(['c.txt', 'd.txt']);

        // Ensure that bulk deletion succeeds when all objects are selected via the checkbox in the table header.
        await objectBrowserPage.selectAllItems();
        await objectBrowserPage.deleteSelectedItems();
        await objectBrowserPage.expectItems([]);

        for (const fileName of ['a.txt', 'b.txt']) {
            await objectBrowserPage.uploadFile(fileName, 'text/plain');
        }

        await objectBrowserPage.createFolder(folderName);
        await objectBrowserPage.clickItem(folderName);
        await objectBrowserPage.waitForItems();

        for (const fileName of ['a.txt', 'b.txt']) {
            await objectBrowserPage.uploadFile(fileName, 'text/plain');
        }

        // Ensure that bulk deletion of individually-selected objects within a subfolder succeeds.
        await objectBrowserPage.selectItem('a.txt');
        await objectBrowserPage.selectItem('b.txt');
        await objectBrowserPage.deleteSelectedItems();
        await objectBrowserPage.expectItems([]);

        // Ensure that no objects in the root directory were affected.
        await objectBrowserPage.clickBreadcrumb(1);
        await objectBrowserPage.waitForItems();
        await objectBrowserPage.expectItems(['testdata', 'a.txt', 'b.txt']);
    });

    test('Nested folder deletion', async ({
        objectBrowserPage,
        bucketsPage,
        navigationMenu,
    }) => {
        const bucketName = uuidv4();
        const folderName = 'testdata';
        const folder2Name = 'testdata2';

        await navigationMenu.clickOnBuckets();
        await bucketsPage.createBucket(bucketName);
        await bucketsPage.openBucket(bucketName);
        await objectBrowserPage.waitForPage();

        // Ensure deleting a folder in the root succeeds.
        await objectBrowserPage.createFolder(folderName);
        await objectBrowserPage.deleteItemByName(folderName);
        await objectBrowserPage.expectItems([]);

        await objectBrowserPage.createFolder(folderName);
        await objectBrowserPage.clickItem(folderName);
        await objectBrowserPage.waitForItems();

        // Ensure deleting a folder in a subfolder succeeds.
        await objectBrowserPage.createFolder(folder2Name);
        await objectBrowserPage.deleteItemByName(folder2Name);
        await objectBrowserPage.expectItems([]);

        await objectBrowserPage.createFolder(folder2Name);
        await objectBrowserPage.clickItem(folder2Name);
        await objectBrowserPage.waitForItems();

        await objectBrowserPage.createFolder(folderName);
        await objectBrowserPage.deleteItemByName(folderName);
        await objectBrowserPage.expectItems([]);
    });

    test('Folder creation and folder drag and drop upload', async ({
        bucketsPage,
        objectBrowserPage,
        navigationMenu,
    }) => {
        const bucketName = uuidv4();
        const folderName = 'testdata';
        const folderPath = join(__dirname, 'testdata');

        await navigationMenu.clickOnBuckets();
        await bucketsPage.createBucket(bucketName);
        await bucketsPage.openBucket(bucketName);

        // Create empty folder using New Folder Button
        await objectBrowserPage.createFolder(folderName);
        await objectBrowserPage.deleteItemByName(folderName);

        // Folder creation with a file inside it
        await objectBrowserPage.uploadFolder(folderPath, folderName);
        await objectBrowserPage.deleteItemByName(folderName);
    });

    test('Folder double-click disallowed', async ({
        bucketsPage,
        objectBrowserPage,
        navigationMenu,
    }) => {
        const bucketName = uuidv4();
        const folderName = 'testdata';

        await navigationMenu.clickOnBuckets();
        await bucketsPage.createBucket(bucketName);
        await bucketsPage.openBucket(bucketName);

        await objectBrowserPage.createFolder(folderName);
        await objectBrowserPage.doubleClickFolder(folderName);
        await objectBrowserPage.checkSingleBreadcrumb(folderName);
    });
});
