// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { SignupPageObjects, SignupPageObjectsV2 } from '@objects/SignupPageObjects';
import type { Page } from '@playwright/test';
import { expect } from '@playwright/test';
import { testConfig } from '../../testConfig';

export class SignupPage {
    constructor(readonly page: Page) {}

    async navigateToSignup(): Promise<void> {
        await this.page.goto(`${testConfig.host}${testConfig.port}/signup`);
    }

    async navigateToPartnerSignup(): Promise<void> {
        await this.page.goto(`${testConfig.host}${testConfig.port}/signup?partner=ix-storj-1`);
    }

    async clickOnPersonalButton(): Promise<void> {
        await this.page.locator(SignupPageObjects.IX_BRANDED_PERSONAL_BUTTON_XPATH).click();
    }

    async clickOnBusinessButton(): Promise<void> {
        await this.page.locator(SignupPageObjects.IX_BRANDED_BUSINESS_BUTTON_XPATH).click();
    }

    async signupApplicationPersonal(name: string, email: string, password: string, branded = false): Promise<void> {
        await this.clickOnPersonalButton();
        await this.page.locator(SignupPageObjects.INPUT_NAME_XPATH).fill(name);
        await this.page.locator(SignupPageObjects.INPUT_EMAIL_XPATH).fill(email);
        await this.page.locator(SignupPageObjects.INPUT_PASSWORD_XPATH).fill(password);
        await this.page.locator(SignupPageObjects.INPUT_RETYPE_PASSWORD_XPATH).fill(password);
        await this.clickOnEveryCheckmark();
        await this.page.locator(branded ? SignupPageObjects.IX_BRANDED_CREATE_ACCOUNT_BUTTON_XPATH : SignupPageObjects.CREATE_ACCOUNT_BUTTON_XPATH).click();
    }

    async clickOnEveryCheckmark(): Promise<void> {
        const checkmarks = await this.page.$$(SignupPageObjects.TOS_CHECKMARK_BUTTON_XPATH);

        for (const checkmark of checkmarks) {
            await checkmark.click({ timeout: 8000 });
        }
    }

    async signupApplicationBusiness(name: string, email: string, password: string, company: string, position: string, branded = false): Promise<void> {
        await this.clickOnBusinessButton();
        await this.page.locator(SignupPageObjects.COMPANY_NAME_INPUT_XPATH).fill(company);
        await this.page.locator(SignupPageObjects.POSITION_INPUT_XPATH).fill(position);
        await this.signupApplicationPersonal(name, email, password, branded);
    }

    async verifySuccessMessage(): Promise<void> {
        await expect(this.page.locator(SignupPageObjects.SIGNUP_SUCCESS_MESSAGE_XPATH)).toBeVisible();
    }

    async verifyIXBrandedHeader(): Promise<void> {
        await expect(this.page.locator(SignupPageObjects.IX_BRANDED_HEADER_TEXT_XPATH)).toBeVisible();
    }

    async verifyIXBrandedSubHeader(): Promise<void> {
        await expect(this.page.locator(SignupPageObjects.IX_BRANDED_SUBHEADER_TEXT_XPATH)).toBeVisible();
    }
}

export class SignupPageV2 {
    constructor(readonly page: Page) {}

    async navigateToLogin(): Promise<void> {
        await this.page.locator(SignupPageObjectsV2.GOTO_LOGIN_PAGE_BUTTON_XPATH).click();
    }

    async navigateToSignup(): Promise<void> {
        await this.page.goto(`${testConfig.host}${testConfig.port}/v2/signup`);
    }

    async signupFirstStep(email: string, password: string): Promise<void> {
        await this.page.locator(SignupPageObjectsV2.INPUT_EMAIL_XPATH).fill(email);
        await this.page.locator(SignupPageObjectsV2.INPUT_PASSWORD_XPATH).fill(password);
        await this.page.locator(SignupPageObjectsV2.INPUT_RETYPE_PASSWORD_XPATH).fill(password);
        await this.page.locator(SignupPageObjectsV2.TOS_CHECKMARK_XPATH).click();
        await this.page.locator(SignupPageObjectsV2.CREATE_ACCOUNT_BUTTON_XPATH).click();
    }

    async verifySuccessMessage(): Promise<void> {
        await expect(this.page.locator(SignupPageObjectsV2.SIGNUP_SUCCESS_MESSAGE_XPATH)).toBeVisible();
    }

    async verifyHeader(): Promise<void> {
        await expect(this.page.locator(SignupPageObjectsV2.HEADER_TEXT_XPATH)).toBeVisible();
    }

    async verifySubheader(): Promise<void> {
        await expect(this.page.locator(SignupPageObjectsV2.SUBHEADER_TEXT_XPATH)).toBeVisible();
    }
}
