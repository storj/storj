// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import {AllProjectsPageObjects} from "@objects/AllProjectsPageObjects";
import type {Page} from '@playwright/test';

export class AllProjectsPage extends AllProjectsPageObjects {
    readonly page: Page;

    constructor(page: Page) {
        super();
        this.page = page;

    }

    async clickOnProject(name: string): Promise<void> {
        await this.page.locator(AllProjectsPageObjects.ALL_PROJECTS_HEADER_TITLE_XPATH).isVisible()
        const listItem = this.page.locator(AllProjectsPageObjects.PROJECT_ITEM_XPATH, {hasText: name})
        const button = listItem.getByText(AllProjectsPageObjects.OPEN_PROJECT_BUTTON_TEXT)
        await button.click()
    }
}
