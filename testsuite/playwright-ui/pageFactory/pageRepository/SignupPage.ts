// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { SignupPageObjects } from '@objects/SignupPageObjects';
import type { Page } from '@playwright/test';
import { expect } from '@playwright/test';
import { testConfig } from '../../testConfig';

export class SignupPage {
    constructor(readonly page: Page) {}

    async navigateToLogin(): Promise<void> {
        await this.page.locator(SignupPageObjects.GOTO_LOGIN_PAGE_BUTTON_XPATH).click();
    }

    async navigateToSignup(): Promise<void> {
        await this.page.goto(`${testConfig.host}:${testConfig.port}/signup`);
    }

    async signupFirstStep(email: string, password: string): Promise<void> {
        await this.page.locator(SignupPageObjects.INPUT_EMAIL_XPATH).fill(email);
        await this.page.locator(SignupPageObjects.INPUT_PASSWORD_XPATH).fill(password);
        await this.page.locator(SignupPageObjects.INPUT_RETYPE_PASSWORD_XPATH).fill(password);
        await this.page.locator(SignupPageObjects.TOS_CHECKMARK_XPATH).click();
        await this.page.locator(SignupPageObjects.CREATE_ACCOUNT_BUTTON_XPATH).click();
    }

    async verifySuccessMessage(): Promise<void> {
        await expect(this.page.locator(SignupPageObjects.SIGNUP_SUCCESS_MESSAGE_XPATH)).toBeVisible();
    }

    async verifyHeader(): Promise<void> {
        await expect(this.page.locator(SignupPageObjects.HEADER_TEXT_XPATH)).toBeVisible();
    }

    async verifySubheader(): Promise<void> {
        await expect(this.page.locator(SignupPageObjects.SUBHEADER_TEXT_XPATH)).toBeVisible();
    }
}
