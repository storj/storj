// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

export class AccessKeysPageObjects {
    static NEW_ACCESS_BUTTON_XPATH = `//button[span[text()=' New Access Key ']]`;
    static CREATE_ACCESS_NAME_INPUT_XPATH = `//input[@placeholder='Enter a name for this access key']`;
    static CREATE_ACCESS_API_KEY_CHIP_XPATH = `//div[text()=' API Key ']`;
    static CREATE_ACCESS_NEXT_BUTTON_XPATH = `//button[span[text()='Next ->']]`;
    static CREATE_ACCESS_CONFIRM_BUTTON_XPATH = `//button[span[text()='Create Access']]`;
    static CREATE_ACCESS_CLOSE_BUTTON_XPATH = `//button[span[text()='Close']]`;
    static ACCESS_ROW_MORE_BUTTON_XPATH = `//button[@title='Access Actions']`;
    static DELETE_ACCESS_BUTTON_XPATH = `//div[div[div[text()=' Delete Access ']]]`;
    static CANNOT_DELETE_ACCESS_DIALOG_TITLE_XPATH = `//div[text()=' Cannot Delete Access Key']`;
    static CANCEL_BUTTON_XPATH = `//button[span[text()='Cancel']]`;
}
