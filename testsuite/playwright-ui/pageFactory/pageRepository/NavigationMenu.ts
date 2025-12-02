// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { NavigationMenuObject } from '@objects/NavigationMenuObject';
import type { Page } from '@playwright/test';

export class NavigationMenu {
    constructor(readonly page: Page) {}

    async clickOnBuckets(): Promise<void> {
        await this.page.locator(NavigationMenuObject.BUCKETS_XPATH).click();
    }

    async clickOnTeam(): Promise<void> {
        await this.page.locator(NavigationMenuObject.TEAM_XPATH).click();
    }

    async clickOnAccessKeys(): Promise<void> {
        await this.page.locator(NavigationMenuObject.ACCESS_KEYS_XPATH).click();
    }

    async enterPassphrase(passphrase: string):Promise<void> {
        await this.page.locator(NavigationMenuObject.PASSPHRASE_INPUT_XPATH).fill(passphrase);
        await this.page.locator(NavigationMenuObject.CONFIRM_ENTER_PASSPHRASE_BUTTON_XPATH).click();
    }

    async switchPassphrase(passphrase: string): Promise<void> {
        await this.page.locator(NavigationMenuObject.PROJECT_SELECT_XPATH).click();
        await this.page.locator(NavigationMenuObject.MANAGE_PASSPHRASE_ACTION_XPATH).click();
        await this.page.locator(NavigationMenuObject.SWITCH_PASSPHRASE_ACTION_XPATH).click();
        await this.page.locator(NavigationMenuObject.PASSPHRASE_INPUT_XPATH).fill(passphrase);
        await this.page.locator(NavigationMenuObject.CONFIRM_SWITCH_PASSPHRASE_BUTTON_XPATH).click();
    }

    async openAccountSettings(): Promise<void> {
        await this.page.locator(NavigationMenuObject.MY_ACCOUNT_BUTTON_XPATH).click();
    }

    async openResources(): Promise<void> {
        await this.page.locator(NavigationMenuObject.RESOURCES_BUTTON_XPATH).click();
    }

    async navigateToAccountSettings(): Promise<void> {
        await this.openAccountSettings();
        await this.page.locator(NavigationMenuObject.ACCOUNT_SETTINGS_MENU_ITEM_XPATH).click();
    }

    async logout(): Promise<void> {
        await this.openAccountSettings();
        await this.page.locator(NavigationMenuObject.SIGN_OUT_MENU_ITEM_XPATH).click();
    }
}
