// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import type { Download, Page, Locator } from '@playwright/test';
import { expect } from '@playwright/test';
import { CommonObjects } from '@objects/CommonObjects';
import { ObjectBrowserPageObjects } from '@objects/ObjectBrowserPageObjects';

export class ObjectBrowserPage {
    constructor(readonly page: Page) {}

    async waitForPage(): Promise<void> {
        await expect(this.page.locator(ObjectBrowserPageObjects.MAIN_VIEW_CLASS)).toBeVisible();

        const tableLoader = this.page.getByRole('alert', { name: 'Loading...' });
        await expect(tableLoader).toBeHidden();
    }

    async waitForItems(): Promise<void> {
        const progressBar = this.page.locator('table').getByRole('progressbar');
        await expect(progressBar).toBeHidden();

        const itemsLoader = this.page.locator(ObjectBrowserPageObjects.LOADING_ITEMS_LABEL_XPATH);
        await expect(itemsLoader).toBeHidden();
    }

    async uploadFile(name: string, format: string): Promise<void> {
        await this.page.locator(ObjectBrowserPageObjects.FILE_INPUT_XPATH).setInputFiles({
            name: name,
            mimeType: format,
            buffer: Buffer.from('Test,T'),
        });
    }

    async expectItems(names: string[]) {
        for (const name of names) {
            await expect(new ItemLocator(this.page, name).getRow()).toBeVisible();
        }
        expect(this.page.locator(ObjectBrowserPageObjects.TABLE_ROW_SELECTOR)).toHaveCount(names.length);
    }

    async uploadFolder(folderPath: string, folderName: string): Promise<void> {
        await this.page.locator(ObjectBrowserPageObjects.FOLDER_INPUT_XPATH).setInputFiles(folderPath);
        await expect(new ItemLocator(this.page, folderName).getRow()).toBeVisible();
    }

    async clickItem(name: string): Promise<void> {
        const itemButton = new ItemLocator(this.page, name).getNameButton();
        await expect(itemButton).toBeVisible();
        await itemButton.click();
    }

    async doubleClickFolder(name: string): Promise<void> {
        await new ItemLocator(this.page, name).getNameButton().dblclick();
    }

    async clickBreadcrumb(index: number): Promise<void> {
        await this.page.locator(ObjectBrowserPageObjects.BREADCRUMB_CLASS).nth(index).click();
    }

    async checkSingleBreadcrumb(name: string): Promise<void> {
        const items = this.page.locator(ObjectBrowserPageObjects.BREADCRUMB_CLASS).getByText(name, { exact: true });
        expect(items).toHaveCount(1);
    }

    async closePreview(): Promise<void> {
        await this.page.locator(ObjectBrowserPageObjects.CLOSE_PREVIEW_MODAL_BUTTON_XPATH).click();
    }

    async downloadFromPreview(): Promise<void> {
        await Promise.all<Download | void>([
            this.page.waitForEvent('download'),
            this.page.locator(ObjectBrowserPageObjects.DOWNLOAD_BUTTON_XPATH).click(),
        ]);
        await expect(this.page.getByText('Keep this download link private.')).toBeVisible();
        // close alert because it obscures preview close button, which can result in test timeout
        await this.page.locator(CommonObjects.CLOSE_ALERT_BUTTON_XPATH).click();
    }

    async verifyObjectMapIsVisible(): Promise<void> {
        await this.page.locator(ObjectBrowserPageObjects.DISTRIBUTION_BUTTON_XPATH).click();
        await this.page.locator(ObjectBrowserPageObjects.OBJECT_MAP_IMAGE_XPATH).isVisible();
        await this.page.locator(ObjectBrowserPageObjects.CLOSE_GEO_DIST_MODAL_BUTTON_XPATH).click();
    }

    async verifyShareObjectLink(): Promise<void> {
        await this.page.locator(ObjectBrowserPageObjects.SHARE_BUTTON_XPATH).click();
        await this.page.locator(ObjectBrowserPageObjects.SHARE_MODAL_NEXT_BUTTON_XPATH).click();
        const title = this.page.locator(ObjectBrowserPageObjects.SHARE_MODAL_PREVIEW_LINK_TITLE_XPATH);
        await expect(title).toBeVisible();
        const copyIcons = await this.page.locator(ObjectBrowserPageObjects.COPY_ICON_BUTTON).all();
        expect(copyIcons).toHaveLength(2);

        await copyIcons[0].click();
        let clipboardContent = await this.page.evaluate(() => navigator.clipboard.readText());
        expect(clipboardContent).toContain('/s/');

        await copyIcons[1].click();
        clipboardContent = await this.page.evaluate(() => navigator.clipboard.readText());
        expect(clipboardContent).toContain('/raw/');

        await this.page.locator(ObjectBrowserPageObjects.CLOSE_SHARE_MODAL_BUTTON_XPATH).click();
    }

    async selectItem(name: string): Promise<void> {
        await new ItemLocator(this.page, name).getSelectionCheckbox().click();
    }

    async selectAllItems(): Promise<void> {
        const checkbox = this.page.locator('thead input[type="checkbox"]');
        await checkbox.uncheck(); // clear existing selection
        await checkbox.check();
    }

    async deleteSelectedItems(): Promise<void> {
        await this.page.locator(ObjectBrowserPageObjects.SNACKBAR_DELETE_BUTTON_SELECTOR).click();
        await this.page.locator(ObjectBrowserPageObjects.CONFIRM_DELETE_BUTTON_SELECTOR).click();

        const selectedRows = this.page.locator(ObjectBrowserPageObjects.TABLE_ROW_SELECTOR, {
            has: this.page.locator('input[type="checkbox"]:checked'),
        });
        await expect(selectedRows).toHaveCount(0);
    }

    async deleteItemByName(name: string): Promise<void> {
        const itemLocator = new ItemLocator(this.page, name);
        await itemLocator.getMoreActionsButton().click();
        await this.page.locator(ObjectBrowserPageObjects.DELETE_ROW_ACTION_BUTTON_XPATH).click();
        await this.page.locator(ObjectBrowserPageObjects.CONFIRM_DELETE_BUTTON_SELECTOR).click();
        await itemLocator.getRow().waitFor({ state: 'hidden' });
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

// ItemColumnIndex is the index of a column within an item row.
enum ItemColumnIndex {
    SelectionCheckbox = 0,
    Name = 1,
    Actions = 5,
}

class ItemLocator {
    private rowLocator: Locator;

    constructor(readonly page: Page, name: string) {
        this.rowLocator = this.page.locator(ObjectBrowserPageObjects.TABLE_ROW_SELECTOR, {
            has: this.page.locator('td').nth(ItemColumnIndex.Name).getByText(name, { exact: true }),
        });
    }

    getRow(): Locator {
        return this.rowLocator;
    }

    getSelectionCheckbox(): Locator {
        return this.getRow().locator('td').nth(ItemColumnIndex.SelectionCheckbox).locator('input[type="checkbox"]');
    }

    getNameButton(): Locator {
        return this.getRow().locator('td').nth(ItemColumnIndex.Name).locator('button');
    }

    getMoreActionsButton(): Locator {
        return this.getRow().locator('td').nth(ItemColumnIndex.Actions).locator('button[title="More Actions"]');
    }
}
