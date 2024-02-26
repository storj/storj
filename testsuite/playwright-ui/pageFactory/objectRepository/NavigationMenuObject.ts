// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class NavigationMenuObject {
    static BUCKETS_XPATH = `//a/div/div[contains(text(),'Buckets')]`;
    static PROJECT_SELECT_XPATH = `//div[div[div[text()='Project']]]`;
    static MANAGE_PASSPHRASE_ACTION_XPATH = `//div/div/div[contains(text(),' Manage Passphrase ')]`;
    static SWITCH_PASSPHRASE_ACTION_XPATH = `//div/div/div/p[contains(text(),'Switch active passphrase')]`;
    static PASSPHRASE_INPUT_XPATH = `//input[@id='Encryption Passphrase']`;
    static CONFIRM_SWITCH_PASSPHRASE_BUTTON_XPATH = `//button[span[text()=' Continue ']]`;
}
