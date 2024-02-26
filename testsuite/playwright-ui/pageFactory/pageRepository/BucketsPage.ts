// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { BucketsPageObjects } from '@objects/BucketsPageObjects';
import type { Page } from '@playwright/test';
import { expect } from '@playwright/test';
import { CommonObjects } from '@objects/CommonObjects';
import { ObjectBrowserPageObjects } from '@objects/ObjectBrowserPageObjects';

export class BucketsPage {
    constructor(readonly page: Page) {}

    async createBucket(name: string): Promise<void> {
        await this.page.locator(BucketsPageObjects.NEW_BUCKET_BUTTON_XPATH).click();
        await this.page.locator(BucketsPageObjects.BUCKET_NAME_INPUT_FIELD_XPATH).fill(name);
        await this.page.locator(BucketsPageObjects.CONFIRM_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
    }

    async openBucketSettings(): Promise<void> {
        await this.page.locator(BucketsPageObjects.BUCKET_ROW_MORE_BUTTON_XPATH).click();
    }

    async verifyBucketDetails(name: string): Promise<void> {
        await this.page.locator(BucketsPageObjects.VIEW_BUCKET_DETAILS_BUTTON_XPATH).click();
        const elems = await this.page.getByText(name).all();
        expect(elems).toHaveLength(2);
    }

    async verifyShareBucket(): Promise<void> {
        await this.page.locator(BucketsPageObjects.SHARE_BUCKET_BUTTON_XPATH).click();
        const loader = this.page.locator(ObjectBrowserPageObjects.SHARE_MODAL_LOADER_CLASS);
        await expect(loader).toBeHidden();
        await this.page.locator(ObjectBrowserPageObjects.COPY_LINK_BUTTON_XPATH).click();
        await this.page.locator('span').filter({ hasText: ObjectBrowserPageObjects.COPIED_TEXT }).isVisible();
        await this.page.locator(CommonObjects.CLOSE_MODAL_BUTTON_XPATH).click();
    }

    async verifyDeleteBucket(name: string): Promise<void> {
        await this.page.locator(BucketsPageObjects.DELETE_BUCKET_BUTTON_XPATH).click();
        await this.page.locator(BucketsPageObjects.CONFIRM_BUTTON_DELETE_BUCKET_FLOW_XPATH).click();
        await expect(this.page.getByRole('button', { name: `Bucket ${name}` })).toBeHidden();
    }
}
