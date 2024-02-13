// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class DashboardPageObjects {
    static WELCOME_TEXT_LOCATOR = `//h2[contains(text(),'Project Stats')]`;
    static ENTER_PASSPHRASE_RADIO_BUTTON_XPATH = `//*[contains(text(),'Enter passphrase')]`;
    static CONTINUE_BUTTON_TEXT = `Continue ->`;
    static PASSPHRASE_INPUT_XPATH = `//input[@id='Encryption Passphrase']`;
    static CHECKMARK_ENTER_PASSPHRASE_XPATH = `//h2[contains(text(),'Yes I understand and saved the passphrase.')]`;
}
