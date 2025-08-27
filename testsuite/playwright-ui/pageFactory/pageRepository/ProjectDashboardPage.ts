// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { expect, Page } from '@playwright/test';

export class ProjectDashboardPage {
    constructor(readonly page: Page) {}

    async verifyDashboardPage(name: string): Promise<void> {
        const locator = this.page.locator(`//h1[text()='Welcome ${name}']`);
        await expect(locator).toBeVisible();
    }
}
