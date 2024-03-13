// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { Page } from '@playwright/test';
import { CommonObjects } from '@objects/CommonObjects';

export class Common {
    constructor(readonly page: Page) {}

    async closeModal(): Promise<void> {
        await this.page.locator(CommonObjects.CLOSE_MODAL_BUTTON_XPATH).click();
    }
}
