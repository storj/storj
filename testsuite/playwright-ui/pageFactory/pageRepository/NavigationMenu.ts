// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { NavigationMenuObject, NavigationMenuObjectV2 } from '@objects/NavigationMenuObject';
import type { Page } from '@playwright/test';

export class NavigationMenu {
    constructor(readonly page: Page) {}

    async clickOnBuckets(): Promise<void> {
        await this.page.locator(NavigationMenuObject.BUCKETS_XPATH).click();
    }
}

export class NavigationMenuV2 {
    constructor(readonly page: Page) {}

    async clickOnBuckets(): Promise<void> {
        await this.page.locator(NavigationMenuObjectV2.BUCKETS_XPATH).click();
    }

    async switchPassphrase(passphrase: string): Promise<void> {
        await this.page.locator(NavigationMenuObjectV2.PROJECT_SELECT_XPATH).click();
        await this.page.locator(NavigationMenuObjectV2.MANAGE_PASSPHRASE_ACTION_XPATH).click();
        await this.page.locator(NavigationMenuObjectV2.SWITCH_PASSPHRASE_ACTION_XPATH).click();
        await this.page.locator(NavigationMenuObjectV2.PASSPHRASE_INPUT_XPATH).fill(passphrase);
        await this.page.locator(NavigationMenuObjectV2.CONFIRM_SWITCH_PASSPHRASE_BUTTON_XPATH).click();
    }
}
