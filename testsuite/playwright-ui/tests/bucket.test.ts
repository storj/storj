// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';

test.describe('Filebrowser + edge services', () => {
    test.beforeEach(async ({loginPage, dashboardPage}, testInfo) => {
        console.log(`Running ${testInfo.title}`);
        await loginPage.navigateToURL();
        await loginPage.loginToApplication();
        await dashboardPage.verifyWelcomeMessage();
    });

    // This test check file download, upload using drag and drop function and basic link-sharing features

    test('File download and upload', async ({navigationMenu, bucketsPage}) => {
        const bucketName = 'uitest1';
        const bucketPassphrase = 'qazwsx';
        const fileName = 'test.txt';

        await bucketsPage.closeModal();
        await navigationMenu.clickOnBuckets();
        await bucketsPage.openBucketByName(bucketName);
        await bucketsPage.enterPassphrase(bucketPassphrase);
        await bucketsPage.clickContinueConfirmPassphrase();

        // Checks for successful download
        await bucketsPage.downloadFileByName(fileName);

        // Checks if the link-sharing buttons work
        await bucketsPage.verifyObjectMapIsVisible();
        await bucketsPage.clickShareButton();
        await bucketsPage.clickCopyLinkButton();
        await bucketsPage.closeModal();

        // Delete old file and upload new with the same file name
        await bucketsPage.deleteFileByName(fileName);
        await bucketsPage.dragAndDropFile(fileName, 'text/csv');
        await bucketsPage.verifyObjectMapIsVisible();
        await bucketsPage.clickShareButton();
        await bucketsPage.clickCopyLinkButton();
    });

    // This test check folder creation, upload using drag and drop function
    test('Folder creation and folder drag and drop upload', async ({navigationMenu, bucketsPage}) => {

        const bucketName = 'testbucket';
        const bucketPassphrase = 'qazwsx';
        const fileName = 'test.txt';
        const folderName = 'test_folder';

        await bucketsPage.closeModal();

        await navigationMenu.clickOnBuckets();
        await bucketsPage.openBucketByName(bucketName);
        await bucketsPage.enterPassphrase(bucketPassphrase);
        await bucketsPage.clickContinueConfirmPassphrase();

        //Create empty folder using New Folder Button
        await bucketsPage.createNewFolder(folderName);
        await bucketsPage.deleteFileByName(folderName);

        //DRAG AND DROP FOLDER creation with a file inside it for next instance of test
        await bucketsPage.dragAndDropFolder(folderName, fileName, 'text/csv');
        await bucketsPage.deleteFileByName(folderName);

    });
    test('Share bucket and bucket details page', async ({navigationMenu, bucketsPage, page}) => {
        const bucketName = 'sharebucket';
        const bucketPassphrase = 'qazwsx';
        const fileName = 'test1.jpeg';

        await bucketsPage.closeModal();

        await navigationMenu.clickOnBuckets();
        await bucketsPage.openBucketByName(bucketName);
        await bucketsPage.enterPassphrase(bucketPassphrase);
        await bucketsPage.clickContinueConfirmPassphrase();

        //open bucket uitest3
        await bucketsPage.openFileByName(fileName);

        // Checks the image preview of the tiny apple png file
        await bucketsPage.verifyImagePreviewIsVisible();
        await bucketsPage.closeModal();

        // Checks for Bucket Detail Header and correct bucket name
        await bucketsPage.openBucketSettings();
        await bucketsPage.clickViewBucketDetails();
        await bucketsPage.verifyDetails(bucketName);
        await page.goBack();

        // Check Bucket Share, see if copy button changed to copied
        await bucketsPage.openBucketSettings();
        await bucketsPage.clickShareBucketButton();
        await bucketsPage.clickCopyButtonShareBucketModal();
        /* toDO: - add check for linksharing link
                 - compare image from linksharing to original
         */
    });
    test('Create and delete bucket', async ({dashboardPage, navigationMenu, bucketsPage}) => {
        const bucketName = 'testdelete';
        const bucketPassphrase = 'qazwsx';

        await dashboardPage.enterOwnPassphraseModal(bucketPassphrase);
        await navigationMenu.clickOnBuckets();

        await bucketsPage.clickNewBucketButton();
        await bucketsPage.enterNewBucketName(bucketName);
        await bucketsPage.clickContinueCreateBucket();
        await navigationMenu.clickOnBuckets();
        await bucketsPage.openBucketDropdownByName(bucketName);
        await bucketsPage.clickDeleteBucketButton();
        await bucketsPage.enterBucketNameDeleteBucket(bucketName);
        await bucketsPage.clickConfirmDeleteButton();
    });


});
