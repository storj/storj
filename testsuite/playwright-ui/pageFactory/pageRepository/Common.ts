// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { expect, Page } from '@playwright/test';
import { CommonObjects, CommonObjectsV2 } from '@objects/CommonObjects';
import { testConfig } from '../../testConfig';

export class Common {
    constructor(readonly page: Page) {}

    async goToAllProjects(): Promise<void> {
        await this.page.goto(`${testConfig.host}${testConfig.port}/all-projects`);
    }

    async closeModal(): Promise<void> {
        await this.page.locator(CommonObjects.CLOSE_MODAL_BUTTON_XPATH).click();
    }

    async waitLoading(): Promise<void> {
        const loaders = await this.page.locator(CommonObjects.LOADER_XPATH).all();
        for (const l of loaders) {
            await expect(l).toBeHidden();
        }
    }
}

export class CommonV2 {
    constructor(readonly page: Page) {}

    async closeModal(): Promise<void> {
        await this.page.locator(CommonObjectsV2.CLOSE_MODAL_BUTTON_XPATH).click();
    }
}
