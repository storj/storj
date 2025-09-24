// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import type { Page } from '@playwright/test';
import { expect } from '@playwright/test';
import { TeamPageObjects } from '@objects/TeamPageObjects';
import { CommonObjects } from '@objects/CommonObjects';

export class TeamPage {
    constructor(readonly page: Page) {}

    async inviteMember(email: string): Promise<void> {
        await this.page.locator(TeamPageObjects.ADD_MEMBERS_BUTTON_XPATH).click();
        await this.page.locator(TeamPageObjects.ADD_MEMBERS_EMAIL_INPUT_XPATH).fill(email);
        await this.page.locator(TeamPageObjects.ADD_MEMBERS_CONFIRM_BUTTON_XPATH).click();
        const dialogTitle = this.page.locator(TeamPageObjects.ADD_MEMBERS_TITLE_XPATH);
        await expect(dialogTitle).toBeHidden();
        await this.page.locator(CommonObjects.CLOSE_ALERT_BUTTON_XPATH).click();
    }

    async confirmOwnerRoleChip(): Promise<void> {
        const loc = this.page.locator(TeamPageObjects.TEAM_ROW_OWNER_CHIP_XPATH);
        await expect(loc).toBeVisible();
    }

    async confirmInvitedRoleChip(): Promise<void> {
        const loc = this.page.locator(TeamPageObjects.TEAM_ROW_INVITED_CHIP_XPATH);
        await expect(loc).toBeVisible();
    }

    async confirmMemberRoleChip(): Promise<void> {
        const loc = this.page.locator(TeamPageObjects.TEAM_ROW_MEMBER_CHIP_XPATH);
        await expect(loc).toBeVisible();
    }

    async joinProject(): Promise<void> {
        const loc = this.page.locator(TeamPageObjects.JOIN_PROJECT_BUTTON_XPATH);
        await expect(loc).toBeVisible();
        await loc.click();
        await this.page.locator(TeamPageObjects.CONFIRM_JOIN_PROJECT_BUTTON_XPATH).click();
    }

    async waitForPage(): Promise<void> {
        const tableLoader = this.page.getByRole('alert', { name: 'Loading items...' });
        await expect(tableLoader).toBeHidden();
    }
}
