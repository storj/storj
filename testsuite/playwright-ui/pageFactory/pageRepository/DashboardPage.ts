// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import {DashboardPageObjects} from "@objects/DashboardPageObjects";
import type {Page} from '@playwright/test';
import {expect} from '@playwright/test';

export class DashboardPage extends DashboardPageObjects {
    readonly page: Page;

    constructor(page: Page) {
        super();
        this.page = page;

    }

    async verifyWelcomeMessage(): Promise<void> {
        await expect(this.page.locator(DashboardPageObjects.WELCOME_TEXT_LOCATOR)).toBeVisible();
    }

    async clickEnterPassphraseRadioButton(): Promise<void> {
        await this.page.locator(DashboardPageObjects.ENTER_PASSPHRASE_RADIO_BUTTON_XPATH).click();
    }

    async clickContinuePassphraseButton(): Promise<void> {
        await this.page.getByText(DashboardPageObjects.CONTINUE_BUTTON_TEXT).click();
    }

    async enterPassphrase(passphrase: string): Promise<void> {
        await this.page.locator(DashboardPageObjects.PASSPHRASE_INPUT_XPATH).fill(passphrase);
    }

    async clickConfirmCheckmark(): Promise<void> {
        await this.page.locator(DashboardPageObjects.CHECKMARK_ENTER_PASSPHRASE_XPATH).click();
    }

    async enterOwnPassphraseModal(passphrase: string): Promise<void> {
        await this.enterPassphrase(passphrase);
        await this.clickContinuePassphraseButton();
    }

}
