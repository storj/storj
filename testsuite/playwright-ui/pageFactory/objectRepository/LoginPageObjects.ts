// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class LoginPageObjects {
    static EMAIL_EDITBOX_ID = `//input[@id='Email Address']`;
    static PASSWORD_EDITBOX_ID = `//input[@id='Password']`;
}

export class LoginPageObjectsV2 {
    static EMAIL_EDITBOX_ID = `//input[@id='Email Address']`;
    static PASSWORD_EDITBOX_ID = `//input[@id='Password']`;
    static CONTINUE_BUTTON_XPATH = `//button[span[text()=' Continue ']]`;

    // SETUP ACCOUNT (FIRST LOGIN)
    static FIRST_STEP_HEADER_XPATH = `//h2[text()='Start by setting up your account']`;
    static PERSONAL_CARD_XPATH = `//div[@id='personal']`;
    static BUSINESS_CARD_XPATH = `//div[@id='business']`;
    static SETUP_SUCCESS_LABEL_XPATH = `//p[text()=' Account Complete ']`;
    static NAME_EDITBOX_ID = `//input[@id='Name']`;
    static FIRST_NAME_EDITBOX_ID = `//input[@id='First Name']`;
    static LAST_NAME_EDITBOX_ID = `//input[@id='Last Name']`;
    static COMPANY_NAME_EDITBOX_ID = `//input[@id='Company Name']`;
    static JOB_ROLE_EDITBOX_ID = `//input[@id='Job Role']`;
}
