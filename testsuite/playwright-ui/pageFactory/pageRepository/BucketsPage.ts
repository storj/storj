// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { BucketsPageObjects } from '@objects/BucketsPageObjects';
import type { Page } from '@playwright/test';
import { expect } from '@playwright/test';

export class BucketsPage {
    constructor(readonly page: Page) {}

    async openBucketByName(name: string): Promise<void> {
        await this.page.getByText(name).click();
    }

    async verifyBucketNotVisible(name: string): Promise<void> {
        await expect(this.page.getByText(name)).toBeHidden();
    }

    async enterPassphrase(phrase: string): Promise<void> {
        await this.page.locator(BucketsPageObjects.ENCRYPTION_PASSPHRASE_XPATH).fill(phrase);
    }

    async clickContinueConfirmPassphrase() {
        await this.page.locator(BucketsPageObjects.CONTINUE_BUTTON_PASSPHRASE_MODAL_XPATH).click();
    }

    async downloadFileByName(name: string): Promise<void> {
        const uiTestFile = this.page.getByText(name);
        await expect(uiTestFile).toBeVisible();
        await uiTestFile.click();
        await Promise.all([
            this.page.waitForEvent('download'),
            this.page.locator(BucketsPageObjects.DOWNLOAD_BUTTON_XPATH).click(),
        ]);
        await expect(this.page.getByText('Keep this download link private.If you want to share, use the Share option.')).toBeVisible();

    }

    async clickShareButton(): Promise<void> {
        await this.page.locator(BucketsPageObjects.SHARE_BUTTON_XPATH).click();
    }

    async clickCopyLinkButton(): Promise<void> {
        await this.page.locator(BucketsPageObjects.COPY_LINK_BUTTON_XPATH).click();
        await this.page.locator('span').filter({ hasText: BucketsPageObjects.COPIED_TEXT }).isVisible();
        await expect(this.page.getByText('Link copied successfully.')).toBeVisible();
    }

    async clickCopyButtonShareBucketModal(): Promise<void> {
        await this.page.locator('span').filter({ hasText: 'Copy link' }).click();
        await this.page.locator('span').filter({ hasText: BucketsPageObjects.COPIED_TEXT }).isVisible();
    }

    async verifyObjectMapIsVisible(): Promise<void> {
        await this.page.locator(BucketsPageObjects.OBJECT_MAP_TEXT_XPATH).isVisible();
        await this.page.locator(BucketsPageObjects.OBJECT_MAP_IMAGE_XPATH).isVisible();
    }

    async verifyImagePreviewIsVisible(): Promise<void> {
        await this.page.getByRole('img', { name: 'preview' }).isVisible();
    }

    async closeModal(): Promise<void> {
        await this.page.locator(BucketsPageObjects.CLOSE_MODAL_BUTTON_XPATH).click();
    }

    async closeFilePreview(): Promise<void> {
        await this.page.locator(BucketsPageObjects.CLOSE_FILE_PREVIEW_BUTTON_XPATH).click();
    }

    async openFileDropdownByName(name: string): Promise<void> {
        const row = await this.page.waitForSelector('*css=tr >> text=' + name);
        const button = await row.$('th:nth-child(5)');
        await button.click();
    }

    async clickDeleteFileButton(): Promise<void> {
        await this.page.locator(BucketsPageObjects.DELETE_BUTTON_XPATH).click();
    }

    async clickNewFolderButton(): Promise<void> {
        await this.page.getByText(BucketsPageObjects.NEW_FOLDER_BUTTON_TEXT).click();
    }

    async createNewFolder(name: string): Promise<void> {
        await this.clickNewFolderButton();
        await this.page.locator(BucketsPageObjects.NEW_FOLDER_NAME_FIELD_XPATH).fill(name);
        await this.page.getByText(BucketsPageObjects.CREATE_FOLDER_BUTTON_TEXT).click();
    }

    async openFileByName(name: string): Promise<void> {
        await this.page.getByText(name).click();
        await this.page.locator(`//p[contains(text(),'${name}')]`).isVisible();
    }

    async openBucketSettings(): Promise<void> {
        await this.page.locator(BucketsPageObjects.BUCKET_SETTINGS_BUTTON_CSS).click();
    }

    async clickViewBucketDetails(): Promise<void> {
        await this.page.locator(BucketsPageObjects.VIEW_BUCKET_DETAILS_BUTTON_CSS).first().click();
    }

    async deleteFileByName(name: string): Promise<void> {
        await this.openFileDropdownByName(name);
        await this.clickDeleteFileButton();
        await this.page.locator(BucketsPageObjects.YES_BUTTON_XPATH).click();
        await this.page.getByText(name).waitFor({ state: 'hidden' });
    }

    async dragAndDropFile(name: string, format: string): Promise<void> {
        await this.page.setInputFiles('input[type=\'file\']', {
            name: name,
            mimeType: format,
            buffer: Buffer.from('Test,T'),
        });
    }

    async dragAndDropFolder(folder: string, filename: string, format: string): Promise<void> {
        await this.page.setInputFiles('input[type=\'file\']', {
            name: folder + '/' + filename,
            mimeType: format,
            buffer: Buffer.from('Test,T'),
        });
        await expect(this.page.getByText(folder)).toBeVisible();
    }

    async verifyDetails(name: string): Promise<void> {
        await this.page.getByRole('cell', { name: name }).isVisible();
    }

    async clickShareBucketButton(): Promise<void> {
        await this.page.locator(BucketsPageObjects.SHARE_BUCKET_BUTTON_XPATH).click();
    }

    async clickNewBucketButton(): Promise<void> {
        await this.page.locator(BucketsPageObjects.NEW_BUCKET_BUTTON_XPATH).click();
    }

    async enterNewBucketName(name: string): Promise<void> {
        await this.page.locator(BucketsPageObjects.BUCKET_NAME_INPUT_FIELD_XPATH).fill(name);
    }

    async clickContinueCreateBucket(): Promise<void> {
        await this.page.locator(BucketsPageObjects.CONTINUE_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
    }

    async clickEnterPassphraseRadioButton(): Promise<void> {
        await this.page.locator(BucketsPageObjects.ENTER_PASSPHRASE_RADIO_BUTTON_XPATH).click();
    }

    async enterNewBucketPassphrase(passphrase: string): Promise<void> {
        await this.page.locator(BucketsPageObjects.PASSPHRASE_INPUT_NEW_BUCKET_XPATH).fill(passphrase);
    }

    async clickConfirmCheckmark(): Promise<void> {
        await this.page.locator(BucketsPageObjects.CHECKMARK_ENTER_PASSPHRASE_XPATH).click();
    }

    async openBucketDropdownByName(name: string): Promise<void> {
        const row = await this.page.waitForSelector('*css=tr >> text=' + name);
        const button = await row.$('th:nth-child(7)');
        await button.click();
    }

    async clickDeleteBucketButton(): Promise<void> {
        await this.page.locator(BucketsPageObjects.DELETE_BUCKET_XPATH).click();
    }

    async enterBucketNameDeleteBucket(name: string): Promise<void> {
        await this.page.locator(BucketsPageObjects.BUCKET_NAME_DELETE_BUCKET_MODAL_XPATH).fill(name);
    }

    async clickConfirmDeleteButton(): Promise<void> {
        await this.page.locator(BucketsPageObjects.CONFIRM_DELETE_BUTTON_XPATH).click();
    }
}
