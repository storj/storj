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
        const header = await this.page.locator(LoginPageObjects.FIRST_STEP_HEADER_XPATH);
        await expect(header).toBeVisible();
    }

    async fillSetupForm(name: string, companyName: string): Promise<void> {
        await this.page.locator(LoginPageObjects.NAME_EDITBOX_ID).fill(name);
        await this.page.locator(LoginPageObjects.COMPANY_NAME_EDITBOX_ID).fill(companyName);
        await this.page.locator(LoginPageObjects.CONTINUE_BUTTON_XPATH).click();
    }

    async createProjectWhenOnboarding(name: string, managedEnc: boolean, skipSelection = false): Promise<void> {
        await this.page.locator(LoginPageObjects.PROJECT_NAME_EDITBOX_ID).fill(name);

        if (!skipSelection) {
            await this.page.locator(LoginPageObjects.PASSPHRASE_MANAGEMENT_SELECT_ID).click();

            if (managedEnc) {
                await this.page.locator(LoginPageObjects.AUTOMATIC_PASSPHRASE_MANAGEMENT_OPTION).click();
            } else {
                await this.page.locator(LoginPageObjects.MANUAL_PASSPHRASE_MANAGEMENT_OPTION).click();
            }
        }

        await this.page.locator(LoginPageObjects.CREATE_PROJECT_BUTTON_XPATH).click();
    }

    async selectFreeTrial() {
        await this.page.locator(LoginPageObjects.FREE_PLAN_XPATH).click();
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
