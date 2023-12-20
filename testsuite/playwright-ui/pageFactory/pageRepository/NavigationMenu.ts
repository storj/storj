// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { NavigationMenuObject } from '@objects/NavigationMenuObject';
import type { Page } from '@playwright/test';

export class NavigationMenu {
    constructor(readonly page: Page) {}

    async clickOnBuckets(): Promise<void> {
        await this.page.locator(NavigationMenuObject.BUCKETS_XPATH).click();
    }
}
