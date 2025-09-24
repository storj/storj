// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import type { Page } from '@playwright/test';
import { expect } from '@playwright/test';
import { AccessKeysPageObjects } from '@objects/AccessKeysPageObjects';

export class AccessKeysPage {
    constructor(readonly page: Page) {}

    async createAPIKey(name: string): Promise<void> {
        await this.page.locator(AccessKeysPageObjects.NEW_ACCESS_BUTTON_XPATH).click();
        await this.page.locator(AccessKeysPageObjects.CREATE_ACCESS_NAME_INPUT_XPATH).fill(name);
        await this.page.locator(AccessKeysPageObjects.CREATE_ACCESS_API_KEY_CHIP_XPATH).click();
        await this.page.locator(AccessKeysPageObjects.CREATE_ACCESS_NEXT_BUTTON_XPATH).click();
        await this.page.locator(AccessKeysPageObjects.CREATE_ACCESS_NEXT_BUTTON_XPATH).click();
        await this.page.locator(AccessKeysPageObjects.CREATE_ACCESS_CONFIRM_BUTTON_XPATH).click();
        await this.page.locator(AccessKeysPageObjects.CREATE_ACCESS_CLOSE_BUTTON_XPATH).click();
    }

    async openAccessSettings(): Promise<void> {
        await this.page.locator(AccessKeysPageObjects.ACCESS_ROW_MORE_BUTTON_XPATH).click();
    }

    async verifyCannotDeleteAccess(): Promise<void> {
        await this.page.locator(AccessKeysPageObjects.DELETE_ACCESS_BUTTON_XPATH).click();
        const loc = this.page.locator(AccessKeysPageObjects.CANNOT_DELETE_ACCESS_DIALOG_TITLE_XPATH);
        await expect(loc).toBeVisible();
        await this.page.locator(AccessKeysPageObjects.CANCEL_BUTTON_XPATH).click();
    }
}
