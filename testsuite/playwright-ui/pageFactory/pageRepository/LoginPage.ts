// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { LoginPageObjects } from '@objects/LoginPageObjects';
import type { Page } from '@playwright/test';
import { expect } from '@playwright/test';
import { testConfig } from '@config/testConfig';

export class LoginPage {
    constructor(readonly page: Page) {}

    async goToLogin(): Promise<void> {
        await this.page.goto(`${testConfig.host}:${testConfig.port}/login`);
    }

    async loginByCreds(email: string, password: string): Promise<void> {
        await this.page.locator(LoginPageObjects.EMAIL_EDITBOX_ID).fill(email);
        await this.page.locator(LoginPageObjects.PASSWORD_EDITBOX_ID).fill(password);
        await this.page.locator(LoginPageObjects.CONTINUE_BUTTON_XPATH).click();
    }

    async verifySetupAccountFirstStep(): Promise<void> {
        const header = this.page.locator(LoginPageObjects.FIRST_STEP_HEADER_XPATH);
        await expect(header).toBeVisible();
    }

    async fillSetupForm(name: string, companyName: string): Promise<void> {
        await this.page.locator(LoginPageObjects.NAME_EDITBOX_ID).fill(name);
        await this.page.locator(LoginPageObjects.COMPANY_NAME_EDITBOX_ID).fill(companyName);
        await this.page.locator(LoginPageObjects.CONTINUE_BUTTON_XPATH).click();
    }

    async selectFreeTrial() {
        await this.page.locator(LoginPageObjects.FREE_PLAN_XPATH).click();
    }

    async selectManagedEnc(automatic: boolean) {
        if (automatic) {
            await this.page.locator(LoginPageObjects.AUTOMATIC_ENC_LABEL_XPATH).click();
        } else {
            await this.page.locator(LoginPageObjects.SELF_MANAGED_ENC_LABEL_XPATH).click();
        }
    }

    async ensureSetupSuccess(): Promise<void> {
        const label = this.page.locator(LoginPageObjects.SETUP_SUCCESS_LABEL_XPATH);
        await expect(label).toBeVisible();
    }

    async finishSetup(): Promise<void> {
        await this.page.locator(LoginPageObjects.CONTINUE_BUTTON_XPATH).nth(1).click();
    }

    async verifyInvalidCredentials(): Promise<void> {
        const error = this.page.locator(LoginPageObjects.ERROR_MESSAGE_XPATH);
        await expect(error).toBeVisible();
    }
}
