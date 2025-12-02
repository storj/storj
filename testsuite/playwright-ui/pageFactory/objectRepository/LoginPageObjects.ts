// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class LoginPageObjects {
    static EMAIL_EDITBOX_ID = `//input[@id='Email Address']`;
    static PASSWORD_EDITBOX_ID = `//input[@id='Password']`;
    static CONTINUE_BUTTON_XPATH = `//button[span[text()=' Continue ']]`;
    static ERROR_MESSAGE_XPATH = `//div[contains(text(), 'Invalid Credentials')]`;

    // SETUP ACCOUNT (FIRST LOGIN)
    static FIRST_STEP_HEADER_XPATH = `//h2[text()='Set up your account']`;
    static FREE_PLAN_XPATH = `//button[@id='free-plan']`;
    static SELF_MANAGED_ENC_LABEL_XPATH = `//button[normalize-space()="Self-managed"]`;
    static AUTOMATIC_ENC_LABEL_XPATH = `//button[normalize-space()="Automatic"]`;
    static SETUP_SUCCESS_LABEL_XPATH = `//p[text()=' Account Complete ']`;
    static NAME_EDITBOX_ID = `//input[@id='Name']`;
    static COMPANY_NAME_EDITBOX_ID = `//input[@id='Company Name']`;
}
