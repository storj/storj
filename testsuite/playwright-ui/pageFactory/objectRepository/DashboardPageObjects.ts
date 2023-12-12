// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class DashboardPageObjects {
    protected static WELCOME_TEXT_LOCATOR = `//h2[contains(text(),'Project Stats')]`;
    protected static ENTER_PASSPHRASE_RADIO_BUTTON_XPATH = `//*[contains(text(),'Enter passphrase')]`;
    protected static CONTINUE_BUTTON_TEXT = `Continue ->`;
    protected static PASSPHRASE_INPUT_XPATH = `//input[@id='Encryption Passphrase']`;
    protected static CHECKMARK_ENTER_PASSPHRASE_XPATH = `//h2[contains(text(),'Yes I understand and saved the passphrase.')]`
}
