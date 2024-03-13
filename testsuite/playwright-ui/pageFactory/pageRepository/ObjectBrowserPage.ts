// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import type { Page } from '@playwright/test';
import { expect } from '@playwright/test';
import { ObjectBrowserPageObjects } from '@objects/ObjectBrowserPageObjects';
import { CommonObjects } from '@objects/CommonObjects';

export class ObjectBrowserPage {
    constructor(readonly page: Page) {}

    async waitLoading(): Promise<void> {
        const loader = this.page.locator(ObjectBrowserPageObjects.LOADING_ITEMS_LABEL_XPATH);
        await expect(loader).toBeHidden();
    }

    async uploadFile(name: string, format: string): Promise<void> {
        await this.page.setInputFiles(ObjectBrowserPageObjects.FILE_INPUT_XPATH, {
            name: name,
            mimeType: format,
            buffer: Buffer.from('Test,T'),
        }, { strict: true });
    }

    async uploadFolder(folder: string, filename: string, format: string): Promise<void> {
        await this.page.setInputFiles(ObjectBrowserPageObjects.FOLDER_INPUT_XPATH, {
            name: folder + '/' + filename,
            mimeType: format,
            buffer: Buffer.from('Test,T'),
        });
        await expect(this.page.getByRole('button', { name: `Foldericon ${folder}` })).toBeVisible();
    }

    async openObjectPreview(name: string, type: string): Promise<void> {
        const uiTestFile = this.page.getByRole('button', { name: `${type}icon ${name}` });
        await expect(uiTestFile).toBeVisible();
        await uiTestFile.click();
    }

    async closePreview(name: string): Promise<void> {
        await this.page.locator('header').filter({ hasText: name }).locator('#Close').click({ force: true });
    }

    async downloadFromPreview(): Promise<void> {
        await Promise.all([
            this.page.waitForEvent('download'),
            this.page.locator(ObjectBrowserPageObjects.DOWNLOAD_BUTTON_XPATH).click(),
        ]);
        await expect(this.page.getByText('Keep this download link private.')).toBeVisible();
    }

    async verifyObjectMapIsVisible(): Promise<void> {
        await this.page.locator(ObjectBrowserPageObjects.DISTRIBUTION_BUTTON_XPATH).click();
        await this.page.locator(ObjectBrowserPageObjects.OBJECT_MAP_IMAGE_XPATH).isVisible();
        await this.page.locator(CommonObjects.CLOSE_MODAL_BUTTON_XPATH).nth(1).click();
    }

    async verifyShareLink(): Promise<void> {
        await this.page.locator(ObjectBrowserPageObjects.SHARE_BUTTON_XPATH).click();
        const loader = this.page.locator(ObjectBrowserPageObjects.SHARE_MODAL_LOADER_CLASS);
        await expect(loader).toBeHidden();
        await this.page.locator(ObjectBrowserPageObjects.COPY_LINK_BUTTON_XPATH).click();
        await this.page.locator('span').filter({ hasText: ObjectBrowserPageObjects.COPIED_TEXT }).isVisible();
        await this.page.locator(CommonObjects.CLOSE_MODAL_BUTTON_XPATH).nth(1).click();
    }

    async deleteSingleObject(): Promise<void> {
        await this.page.locator(ObjectBrowserPageObjects.OBJECT_ROW_MORE_BUTTON_XPATH).click();
        await this.page.locator(ObjectBrowserPageObjects.DELETE_ROW_ACTION_BUTTON_XPATH).click();
        await this.page.locator(ObjectBrowserPageObjects.CONFIRM_DELETE_BUTTON_XPATH).click();
    }

    async deleteObjectByName(name: string, type: string): Promise<void> {
        await this.deleteSingleObject();
        await this.page.getByRole('button', { name: `${type}icon ${name}` }).waitFor({ state: 'hidden' });
    }

    async createFolder(folderName: string): Promise<void> {
        await this.page.locator(ObjectBrowserPageObjects.CREATE_FOLDER_BUTTON_XPATH).click();
        await this.page.locator(ObjectBrowserPageObjects.FOLDER_NAME_INPUT_XPATH).fill(folderName);
        await this.page.locator(ObjectBrowserPageObjects.CONFIRM_CREATE_FOLDER_BUTTON_XPATH).click();
    }

    async verifyImagePreviewIsVisible(): Promise<void> {
        await this.page.getByRole('img', { name: 'preview' }).isVisible();
    }
}
