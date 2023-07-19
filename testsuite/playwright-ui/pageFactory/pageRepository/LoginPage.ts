// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import {LoginPageObjects} from "@objects/LoginPageObjects";
import type {Page} from '@playwright/test';
import {testConfig} from '../../testConfig';

export class LoginPage extends LoginPageObjects {
    readonly page: Page;

    constructor(page: Page) {
        super();
        this.page = page;
    }

    async navigateToURL(): Promise<void> {
        await this.page.goto(testConfig.host+testConfig.port);
    }

    async loginToApplication(): Promise<void> {
        await this.page.locator(LoginPageObjects.EMAIL_EDITBOX_ID).fill(testConfig.username);
        await this.page.locator(LoginPageObjects.PASSWORD_EDITBOX_ID).fill(testConfig.password);
        await this.page.locator(LoginPageObjects.SIGN_IN_BUTTON_XPATH).click();
    }
}
