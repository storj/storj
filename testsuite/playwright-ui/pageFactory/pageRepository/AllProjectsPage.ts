// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { AllProjectsPageObjects } from '@objects/AllProjectsPageObjects';
import type { Page } from '@playwright/test';

export class AllProjectsPage {
    constructor(readonly page: Page) {}

    async createProject(name: string): Promise<void> {
        await this.page.locator(AllProjectsPageObjects.CREATE_PROJECT_BUTTON_XPATH).click();
        await this.page.locator(AllProjectsPageObjects.NEW_PROJECT_NAME_FIELD_XPATH).fill(name);
        await this.page.locator(AllProjectsPageObjects.CONFIRM_CREATE_PROJECT_BUTTON_XPATH).click();
    }
}
