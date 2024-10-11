// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { LoginPageObjects } from '@objects/LoginPageObjects';
import type { Page } from '@playwright/test';
import { expect } from '@playwright/test';

export class LoginPage {
    constructor(readonly page: Page) {}

    async loginByCreds(email: string, password: string): Promise<void> {
        await this.page.locator(LoginPageObjects.EMAIL_EDITBOX_ID).fill(email);
        await this.page.locator(LoginPageObjects.PASSWORD_EDITBOX_ID).fill(password);
        await this.page.locator(LoginPageObjects.CONTINUE_BUTTON_XPATH).click();
    }

    async verifySetupAccountFirstStep(): Promise<void> {
        const header = this.page.locator(LoginPageObjects.FIRST_STEP_HEADER_XPATH);
        await expect(header).toBeVisible();
    }

    async choosePersonalAccSetup(): Promise<void> {
        await this.page.locator(LoginPageObjects.PERSONAL_CARD_XPATH).click();
    }

    async chooseBusinessAccSetup(): Promise<void> {
        await this.page.locator(LoginPageObjects.BUSINESS_CARD_XPATH).click();
    }

    async fillPersonalSetupForm(name: string): Promise<void> {
        await this.page.locator(LoginPageObjects.NAME_EDITBOX_ID).fill(name);
        await this.page.locator(LoginPageObjects.CONTINUE_BUTTON_XPATH).click();
    }

    async fillBusinessSetupForm(firstName: string, lastName: string, companyName: string): Promise<void> {
        await this.page.locator(LoginPageObjects.FIRST_NAME_EDITBOX_ID).fill(firstName);
        await this.page.locator(LoginPageObjects.LAST_NAME_EDITBOX_ID).fill(lastName);
        await this.page.locator(LoginPageObjects.COMPANY_NAME_EDITBOX_ID).fill(companyName);
        await this.page.locator(LoginPageObjects.JOB_ROLE_EDITBOX_ID).click({ force: true }); // force is necessary to open v-select menu
        await this.page.locator(LoginPageObjects.JOB_ROLE_SELECTION_XPATH).click();
        await this.page.locator(LoginPageObjects.CONTINUE_BUTTON_XPATH).click();
    }

    async selectFreeTrial() {
        await this.page.locator(LoginPageObjects.FREE_PLAN_XPATH).click();
        await this.page.locator(LoginPageObjects.ACTIVATE_XPATH).click();
    }

    async ensureSetupSuccess(): Promise<void> {
        const label = this.page.locator(LoginPageObjects.SETUP_SUCCESS_LABEL_XPATH);
        await expect(label).toBeVisible();
    }

    async finishSetup(): Promise<void> {
        await this.page.locator(LoginPageObjects.CONTINUE_BUTTON_XPATH).nth(1).click();
    }
}
