// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class SignupPageObjects {
    // SIGNUP
    static INPUT_EMAIL_XPATH = `//input[@id='Email Address']`;
    static INPUT_PASSWORD_XPATH = `//input[@id='Password']`;
    static INPUT_RETYPE_PASSWORD_XPATH = `//input[@id='Retype Password']`;
    static TOS_CHECKMARK_XPATH = `//input[@id='Terms checkbox']`;
    static CREATE_ACCOUNT_BUTTON_XPATH = `//button[span[text()='Get Started']]`;
    static HEADER_TEXT_XPATH = `//div[contains(text(),'Experience better cloud storage for your business')]`;
    static SUBHEADER_TEXT_XPATH = `//div[contains(text(),'Start for free and get unparalleled performance and')]`;

    // SIGNUP SUCCESS PAGE
    static SIGNUP_SUCCESS_MESSAGE_XPATH = `//h2[contains(text(),'You are almost ready to use Storj')]`;
    static GOTO_LOGIN_PAGE_BUTTON_XPATH = `//a[contains(text(),'Go to login page')]`;
}
