// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { BucketsPageObjects } from '@objects/BucketsPageObjects';
import type { Page } from '@playwright/test';
import { expect } from '@playwright/test';
import { ObjectBrowserPageObjects } from '@objects/ObjectBrowserPageObjects';

export class BucketsPage {
    constructor(readonly page: Page) {}

    async createBucket(name: string): Promise<void> {
        await this.createBucketNameStep(name);
        await this.createBucketPlacementStep(BucketsPageObjects.NEW_BUCKET_GLOBAL_PLACEMENT_BUTTON_XPATH);
        await this.page.locator(BucketsPageObjects.OBJECT_LOCK_TITLE_CREATE_BUCKET_FLOW_XPATH).isVisible();
        await this.page.locator(BucketsPageObjects.NEXT_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.VERSIONING_TITLE_CREATE_BUCKET_FLOW_XPATH).isVisible();
        await this.page.locator(BucketsPageObjects.NEXT_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.CONFIRM_SUBTITLE_CREATE_BUCKET_FLOW_XPATH).isVisible();
        await this.page.locator(BucketsPageObjects.CONFIRM_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.CLOSE_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
    }

    async createBucketWithVersioning(name: string): Promise<void> {
        await this.createBucketNameStep(name);
        await this.createBucketPlacementStep(BucketsPageObjects.NEW_BUCKET_GLOBAL_PLACEMENT_BUTTON_XPATH);
        await this.page.locator(BucketsPageObjects.OBJECT_LOCK_TITLE_CREATE_BUCKET_FLOW_XPATH).isVisible();
        await this.page.locator(BucketsPageObjects.NEXT_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.VERSIONING_TITLE_CREATE_BUCKET_FLOW_XPATH).isVisible();
        await this.page.locator(BucketsPageObjects.ENABLE_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.NEXT_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.CONFIRM_SUBTITLE_CREATE_BUCKET_FLOW_XPATH).isVisible();
        await this.verifyEnabledStatus(1);
        await this.page.locator(BucketsPageObjects.CONFIRM_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.CLOSE_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
    }

    async createBucketWithObjectLock(name: string): Promise<void> {
        await this.createBucketNameStep(name);
        await this.createBucketPlacementStep(BucketsPageObjects.NEW_BUCKET_GLOBAL_PLACEMENT_BUTTON_XPATH);
        await this.page.locator(BucketsPageObjects.OBJECT_LOCK_TITLE_CREATE_BUCKET_FLOW_XPATH).isVisible();
        await this.page.locator(BucketsPageObjects.YES_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.NEXT_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.VERSIONING_TITLE_CREATE_BUCKET_FLOW_XPATH).isVisible();
        await this.page.locator(BucketsPageObjects.NEXT_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.CONFIRM_SUBTITLE_CREATE_BUCKET_FLOW_XPATH).isVisible();
        await this.verifyEnabledStatus(2);
        await this.page.locator(BucketsPageObjects.CONFIRM_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.CLOSE_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
    }

    async createBucketWithPlacement(name: string, placementButton: string, expectedPlacement: string): Promise<void> {
        await this.createBucketNameStep(name);
        await this.createBucketPlacementStep(placementButton);
        await this.page.locator(BucketsPageObjects.OBJECT_LOCK_TITLE_CREATE_BUCKET_FLOW_XPATH).isVisible();
        await this.page.locator(BucketsPageObjects.NEXT_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.VERSIONING_TITLE_CREATE_BUCKET_FLOW_XPATH).isVisible();
        await this.page.locator(BucketsPageObjects.NEXT_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.CONFIRM_SUBTITLE_CREATE_BUCKET_FLOW_XPATH).isVisible();
        await this.verifyLocation(expectedPlacement);
        await this.page.locator(BucketsPageObjects.CONFIRM_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
        await this.page.locator(BucketsPageObjects.CLOSE_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
    }

    async createBucketNameStep(name: string): Promise<void> {
        await this.page.locator(BucketsPageObjects.NEW_BUCKET_BUTTON_XPATH).click();
        await this.page.locator(BucketsPageObjects.BUCKET_NAME_INPUT_FIELD_XPATH).fill(name);
        await this.page.locator(BucketsPageObjects.NEXT_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
    }

    async createBucketPlacementStep(placementButton: string): Promise<void> {
        await this.page.locator(BucketsPageObjects.SELF_SERVE_PLACEMENT_TITLE_CREATE_BUCKET_FLOW_XPATH).isVisible();
        await this.page.locator(placementButton).click();
        await this.page.locator(BucketsPageObjects.NEXT_BUTTON_CREATE_BUCKET_FLOW_XPATH).click();
    }

    async verifyLocation(expected: string): Promise<void> {
        const elems = await this.page.locator(`//div[text()='${expected}']`).all();
        // 2 is expected here because the location appears twice in the DOM tree.
        expect(elems).toHaveLength(2);
    }

    async openBucket(name: string): Promise<void> {
        await this.page.locator(`//td[button[span[div[p[text()='${name}']]]]]`).click();
    }

    async openBucketSettings(): Promise<void> {
        await this.page.locator(BucketsPageObjects.BUCKET_ROW_MORE_BUTTON_XPATH).click();
    }

    async openBucketDetails(): Promise<void> {
        await this.page.locator(BucketsPageObjects.VIEW_BUCKET_DETAILS_BUTTON_XPATH).click();
    }

    async closeBucketDetails(): Promise<void> {
        await this.page.locator(BucketsPageObjects.CLOSE_DETAILS_MODAL_BUTTON_XPATH).click();
    }

    async verifyEnabledStatus(expectedItems: number): Promise<void> {
        const elems = await this.page.locator(BucketsPageObjects.ENABLE_LABEL_CREATE_BUCKET_FLOW_XPATH).all();
        expect(elems).toHaveLength(expectedItems);
    }

    async verifyBucketDetails(name: string): Promise<void> {
        await this.page.locator(BucketsPageObjects.VIEW_BUCKET_DETAILS_BUTTON_XPATH).click();
        const elems = await this.page.getByText(name).all();
        expect(elems).toHaveLength(2);
        await this.closeBucketDetails();
    }

    async verifyShareBucket(): Promise<void> {
        await this.page.locator(BucketsPageObjects.SHARE_BUCKET_BUTTON_XPATH).click();
        await this.page.locator(ObjectBrowserPageObjects.SHARE_MODAL_NEXT_BUTTON_XPATH).click();
        const title = this.page.locator(ObjectBrowserPageObjects.SHARE_MODAL_PREVIEW_LINK_TITLE_XPATH);
        await expect(title).toBeVisible();
        await this.page.locator(ObjectBrowserPageObjects.COPY_LINK_BUTTON_XPATH).click();
        await this.page.locator('span').filter({ hasText: ObjectBrowserPageObjects.COPIED_TEXT }).isVisible();
        await this.page.locator(ObjectBrowserPageObjects.CLOSE_SHARE_MODAL_BUTTON_XPATH).click();
    }

    async verifyCannotDeleteBucket(): Promise<void> {
        await this.page.locator(BucketsPageObjects.DELETE_BUCKET_BUTTON_XPATH).click();
        const loc = this.page.locator(BucketsPageObjects.CANNOT_DELETE_BUCKET_DIALOG_TITLE_XPATH);
        await expect(loc).toBeVisible();
        await this.page.locator(BucketsPageObjects.CANCEL_BUTTON_XPATH).click();
    }

    async verifyDeleteBucket(name: string): Promise<void> {
        await this.page.locator(BucketsPageObjects.DELETE_BUCKET_BUTTON_XPATH).click();
        await this.page.locator(BucketsPageObjects.CONFIRM_DELETE_INPUT_FIELD_XPATH).fill('DELETE');
        await this.page.locator(BucketsPageObjects.CONFIRM_BUTTON_DELETE_BUCKET_FLOW_XPATH).click();
        await expect(this.page.getByRole('button', { name: `Bucket ${name}` })).toBeHidden();
    }
}
