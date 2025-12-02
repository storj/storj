// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class NavigationMenuObject {
    static BUCKETS_XPATH = `//a/div/div[contains(text(),'Browse')]`;
    static PROJECT_SELECT_XPATH = `//div[div[div[text()='Project']]]`;
    static MANAGE_PASSPHRASE_ACTION_XPATH = `//div/div/div[contains(text(),' Manage Passphrase ')]`;
    static SWITCH_PASSPHRASE_ACTION_XPATH = `//div/div/div/p[contains(text(),'Switch active passphrase')]`;
    static PASSPHRASE_INPUT_XPATH = `//input[@id='Encryption Passphrase']`;
    static CONFIRM_SWITCH_PASSPHRASE_BUTTON_XPATH = `//button[span[text()=' Continue ']]`;
    static CONFIRM_ENTER_PASSPHRASE_BUTTON_XPATH = `//button[span[text()='Continue ->']]`;
    static MY_ACCOUNT_BUTTON_XPATH = `//button[span[text()=' My Account ']]`;
    static RESOURCES_BUTTON_XPATH = `//div[text()='Resources']`;
    static ACCOUNT_SETTINGS_MENU_ITEM_XPATH = `//div[text()=' Settings ']`;
    static SIGN_OUT_MENU_ITEM_XPATH = `//div[text()=' Sign Out ']`;
    static TEAM_XPATH = `//a/div/div[contains(text(),'Team')]`;
    static ACCESS_KEYS_XPATH = `//a/div/div[contains(text(),'Access Keys')]`;
}
