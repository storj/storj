// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class SignupPageObjects {
    // SIGNUP
    static INPUT_NAME_XPATH = `//input[@id='Full Name']`;
    static INPUT_EMAIL_XPATH = `//input[@id='Email Address']`;
    static INPUT_PASSWORD_XPATH = `//input[@id='Password']`;
    static INPUT_RETYPE_PASSWORD_XPATH = `//input[@id='Retype Password']`;
    static TOS_CHECKMARK_BUTTON_XPATH = `.checkmark-container`;
    static CREATE_ACCOUNT_BUTTON_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Get Started')]`;

    // SIGNUP SUCCESS PAGE
    static SIGNUP_SUCCESS_MESSAGE_XPATH = `//h2[contains(text(),"You're almost there!")]`;
    static GOTO_LOGIN_PAGE_BUTTON_XPATH = `//a[contains(text(),'Go to Login page')]`;

    // IX BRANDED SIGNUP
    static IX_BRANDED_CREATE_ACCOUNT_BUTTON_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Create an iX-Storj Account')]`;
    static IX_BRANDED_HEADER_TEXT_XPATH = `//h1[contains(text(),'Globally Distributed Storage for TrueNAS')]`;
    static IX_BRANDED_SUBHEADER_TEXT_XPATH = `//p[contains(text(),'iX and Storj have partnered to offer a secure, hig')]`;

    // BUSINESS TAB
    static IX_BRANDED_PERSONAL_BUTTON_XPATH = `//li[contains(text(),'Personal')]`;
    static IX_BRANDED_BUSINESS_BUTTON_XPATH = `//li[contains(text(),'Business')]`;
    static COMPANY_NAME_INPUT_XPATH = `//input[@id='Company Name']`;
    static POSITION_INPUT_XPATH = `//input[@id='Position']`;
}
