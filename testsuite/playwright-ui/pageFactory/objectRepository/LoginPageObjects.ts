// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class LoginPageObjects {
    static EMAIL_EDITBOX_ID = `//input[@id='Email Address']`;
    static PASSWORD_EDITBOX_ID = `//input[@id='Password']`;
    static CONTINUE_BUTTON_XPATH = `//button[span[text()=' Continue ']]`;
    static ERROR_MESSAGE_XPATH = `//div[contains(text(), 'Invalid Credentials')]`;

    // SETUP ACCOUNT (FIRST LOGIN)
    static FIRST_STEP_HEADER_XPATH = `//h2[normalize-space()='Set up your account']`;
    static FREE_PLAN_XPATH = `//button[@id='free-plan']`;
    static SETUP_SUCCESS_LABEL_XPATH = `//p[text()=' Account Complete ']`;
    static NAME_EDITBOX_ID = `//input[@id='Name']`;
    static COMPANY_NAME_EDITBOX_ID = `//input[@id='Company Name']`;
    static PROJECT_NAME_EDITBOX_ID = `//input[@id='Project Name']`;
    static PASSPHRASE_MANAGEMENT_SELECT_ID = `//input[@id='Select Passphrase Management Mode']/ancestor::div[contains(@class,'v-select')]`;
    static AUTOMATIC_PASSPHRASE_MANAGEMENT_OPTION = `//div[contains(@class,'v-list-item-title') and normalize-space()='Automatic (Default)']`;
    static MANUAL_PASSPHRASE_MANAGEMENT_OPTION = `//div[contains(@class,'v-list-item-title') and normalize-space()='Self-Managed']`;
    static CREATE_PROJECT_BUTTON_XPATH = `//button[@id='Create Project']`;
}
