// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { AccountSettingsObjects } from '@objects/AccountSettingsObjects';
import { expect, Page } from '@playwright/test';

export class AccountSettingsPage {
    constructor(readonly page: Page) {}

    async checkName(expectedName: string): Promise<void> {
        const locator = this.page.locator(`//div[text()='${expectedName}']`);
        await expect(locator).toBeVisible();
    }

    async changeName(newName: string): Promise<void> {
        await this.page.locator(AccountSettingsObjects.EDIT_NAME_BUTTON_XPATH).click();
        const nameTitle = this.page.locator(AccountSettingsObjects.EDIT_NAME_DIALOG_TITLE_XPATH);
        await expect(nameTitle).toBeVisible();
        await this.page.locator(AccountSettingsObjects.EDIT_NAME_INPUT_XPATH).fill(newName);
        await this.page.locator(AccountSettingsObjects.EDIT_NAME_SAVE_BUTTON_XPATH).click();
    }
}
