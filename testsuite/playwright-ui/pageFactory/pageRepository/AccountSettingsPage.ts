// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { AccountSettingsObjects } from '@objects/AccountSettingsObjects';
import { expect, Page } from '@playwright/test';
import { CommonObjects } from '@objects/CommonObjects';

export class AccountSettingsPage {
    constructor(readonly page: Page) {}

    async checkName(expectedName: string): Promise<void> {
        const locator = this.page.locator(`//div[text()='${expectedName}']`);
        await expect(locator).toBeVisible();
    }

    async changeName(newName: string): Promise<void> {
        await this.page.locator(AccountSettingsObjects.EDIT_NAME_BUTTON_XPATH).click();
        const dialogTitle = this.page.locator(AccountSettingsObjects.EDIT_NAME_DIALOG_TITLE_XPATH);
        await expect(dialogTitle).toBeVisible();
        await this.page.locator(AccountSettingsObjects.EDIT_NAME_INPUT_XPATH).fill(newName);
        await this.page.locator(AccountSettingsObjects.SAVE_BUTTON_XPATH).click();
    }

    async changePassword(oldPassword: string, newPassword: string): Promise<void> {
        await this.page.locator(AccountSettingsObjects.CHANGE_PASSWORD_BUTTON_XPATH).click();
        const dialogTitle = this.page.locator(AccountSettingsObjects.CHANGE_PASSWORD_DIALOG_TITLE_XPATH);
        await expect(dialogTitle).toBeVisible();
        await this.page.locator(AccountSettingsObjects.CHANGE_PASSWORD_CURRENT_INPUT_XPATH).fill(oldPassword);
        await this.page.locator(AccountSettingsObjects.CHANGE_PASSWORD_NEW_INPUT_XPATH).fill(newPassword);
        await this.page.locator(AccountSettingsObjects.CHANGE_PASSWORD_CONFIRM_INPUT_XPATH).fill(newPassword);
        await this.page.locator(AccountSettingsObjects.SAVE_BUTTON_XPATH).click();
        await this.page.locator(CommonObjects.CLOSE_ALERT_BUTTON_XPATH).click();
    }

    async changeSessionTimeout(oldTimeout: string, newTimeout: string): Promise<void> {
        await this.page.locator(AccountSettingsObjects.CHANGE_SESSION_TIMEOUT_BUTTON_XPATH).click();
        const titles = await this.page.locator(AccountSettingsObjects.CHANGE_SESSION_TIMEOUT_DIALOG_TITLE_XPATH).all();
        expect(titles).toHaveLength(2); // there are two identical titles in the DOM tree.
        await this.page.locator(`//div[span[text()='${oldTimeout}']]`).click();
        await this.page.locator(`//div[text()='${newTimeout}']`).click();
        await this.page.locator(AccountSettingsObjects.SAVE_BUTTON_XPATH).click();
        await this.page.locator(CommonObjects.CLOSE_ALERT_BUTTON_XPATH).click();
    }

    async verifySessionTimeout(expectedLabel: string): Promise<void> {
        const locator = this.page.locator(`//div[text()='${expectedLabel}']`);
        await expect(locator).toBeVisible();
    }
}
