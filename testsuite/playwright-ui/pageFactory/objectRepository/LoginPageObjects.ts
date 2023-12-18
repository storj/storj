// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class LoginPageObjects {
    static EMAIL_EDITBOX_ID = `//input[@id='Email Address']`;
    static PASSWORD_EDITBOX_ID = `//input[@id='Password']`;
    static SIGN_IN_BUTTON_XPATH = `//div[contains(@class, 'container login-area__content-area__container__button') and contains(.//span, 'Sign In')]`;
}
