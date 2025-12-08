// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class SignupPageObjects {
    // SIGNUP
    static INPUT_EMAIL_XPATH = `//input[@id='Email Address']`;
    static INPUT_PASSWORD_XPATH = `//input[@id='Password']`;
    static INPUT_RETYPE_PASSWORD_XPATH = `//input[@id='Retype Password']`;
    static TOS_CHECKMARK_XPATH = `//input[@id='Terms checkbox']`;
    static CREATE_ACCOUNT_BUTTON_XPATH = `//button[span[text()=' Start your free trial ']]`;
    static HEADER_TEXT_XPATH = `//h1[.='Start using Storj today.']`;
    static SUBHEADER_TEXT_XPATH = `//p[contains(text(),'Whether migrating your data or just testing out')]`;

    // SIGNUP SUCCESS PAGE
    static SIGNUP_SUCCESS_MESSAGE_XPATH = `//h2[contains(text(),'You are almost ready to use')]`;
    static GOTO_LOGIN_PAGE_BUTTON_XPATH = `//a[contains(text(),'Go to Login')]`;
}
