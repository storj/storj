// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { LoginPageObjects, LoginPageObjectsV2 } from '@objects/LoginPageObjects';
import type { Page } from '@playwright/test';
import { testConfig } from '../../testConfig';
import { expect } from '@playwright/test';

export class LoginPage {
    constructor(readonly page: Page) {}

    async navigateToURL(): Promise<void> {
        await this.page.goto(`${testConfig.host}${testConfig.port}/login`);
    }

    async loginByCreds(email: string, password: string): Promise<void> {
        await this.page.locator(LoginPageObjects.EMAIL_EDITBOX_ID).fill(email);
        await this.page.locator(LoginPageObjects.PASSWORD_EDITBOX_ID).fill(password);
        await this.page.locator('span').filter({ hasText: 'Sign In' }).click();
    }

    async loginToApplication(): Promise<void> {
        await this.page.locator(LoginPageObjects.EMAIL_EDITBOX_ID).fill(testConfig.username);
        await this.page.locator(LoginPageObjects.PASSWORD_EDITBOX_ID).fill(testConfig.password);
        await this.page.locator('span').filter({ hasText: 'Sign In' }).click();
    }
}

export class LoginPageV2 {
    constructor(readonly page: Page) {}

    async loginByCreds(email: string, password: string): Promise<void> {
        await this.page.locator(LoginPageObjectsV2.EMAIL_EDITBOX_ID).fill(email);
        await this.page.locator(LoginPageObjectsV2.PASSWORD_EDITBOX_ID).fill(password);
        await this.page.locator(LoginPageObjectsV2.CONTINUE_BUTTON_XPATH).click();
    }

    async verifySetupAccountFirstStep(): Promise<void> {
        const header = this.page.locator(LoginPageObjectsV2.FIRST_STEP_HEADER_XPATH);
        await expect(header).toBeVisible();
    }

    async choosePersonalAccSetup(): Promise<void> {
        await this.page.locator(LoginPageObjectsV2.PERSONAL_CARD_XPATH).click();
    }

    async chooseBusinessAccSetup(): Promise<void> {
        await this.page.locator(LoginPageObjectsV2.BUSINESS_CARD_XPATH).click();
    }

    async fillPersonalSetupForm(name: string): Promise<void> {
        await this.page.locator(LoginPageObjectsV2.NAME_EDITBOX_ID).fill(name);
        await this.page.locator(LoginPageObjectsV2.CONTINUE_BUTTON_XPATH).click();
    }

    async fillBusinessSetupForm(firstName: string, lastName: string, companyName: string, jobRole: string): Promise<void> {
        await this.page.locator(LoginPageObjectsV2.FIRST_NAME_EDITBOX_ID).fill(firstName);
        await this.page.locator(LoginPageObjectsV2.LAST_NAME_EDITBOX_ID).fill(lastName);
        await this.page.locator(LoginPageObjectsV2.COMPANY_NAME_EDITBOX_ID).fill(companyName);
        await this.page.locator(LoginPageObjectsV2.JOB_ROLE_EDITBOX_ID).fill(jobRole);
        await this.page.locator(LoginPageObjectsV2.CONTINUE_BUTTON_XPATH).click();
    }

    async ensureSetupSuccess(): Promise<void> {
        const label = this.page.locator(LoginPageObjectsV2.SETUP_SUCCESS_LABEL_XPATH);
        await expect(label).toBeVisible();
    }

    async finishSetup(): Promise<void> {
        await this.page.locator(LoginPageObjectsV2.CONTINUE_BUTTON_XPATH).nth(1).click();
    }
}
