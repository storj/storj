// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import {NavigationMenuObject} from "@objects/NavigationMenuObject";
import type {Page} from '@playwright/test';

export class NavigationMenu extends NavigationMenuObject {
    readonly page: Page;

    constructor(page: Page) {
        super();
        this.page = page;
    }

    async clickOnBuckets(): Promise<void> {
        await this.page.getByRole('link', {name: NavigationMenuObject.BUCKETS_XPATH}).click();

    }
}
