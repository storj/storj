// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

export class AccountSettingsObjects {
    static EDIT_NAME_BUTTON_XPATH = `//button[span[text()=' Edit Name ']]`;
    static EDIT_NAME_DIALOG_TITLE_XPATH = `//div[text()='Edit Name']`;
    static EDIT_NAME_INPUT_XPATH = `//input[@placeholder='Enter your name']`;
    static SAVE_BUTTON_XPATH = `//button[span[text()=' Save ']]`;
    static CHANGE_PASSWORD_BUTTON_XPATH = `//button[span[text()=' Change Password ']]`;
    static CHANGE_PASSWORD_DIALOG_TITLE_XPATH = `//div[text()='Change Password']`;
    static CHANGE_PASSWORD_CURRENT_INPUT_XPATH = `//input[@placeholder='Enter your current password']`;
    static CHANGE_PASSWORD_NEW_INPUT_XPATH = `//input[@placeholder='Enter a new password']`;
    static CHANGE_PASSWORD_CONFIRM_INPUT_XPATH = `//input[@placeholder='Enter the new password again']`;
    static CHANGE_SESSION_TIMEOUT_BUTTON_XPATH = `//button[span[text()=' Change Timeout ']]`;
    static CHANGE_SESSION_TIMEOUT_DIALOG_TITLE_XPATH = `//div[text()='Session Timeout']`;
}
