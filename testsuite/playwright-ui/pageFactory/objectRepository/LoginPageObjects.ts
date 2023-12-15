// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class LoginPageObjects {
    protected static EMAIL_EDITBOX_ID = `//input[@id='Email Address']`;
    protected static PASSWORD_EDITBOX_ID = `//input[@id='Password']`;
    protected static SIGN_IN_BUTTON_XPATH = `//div[contains(@class, 'container login-area__content-area__container__button') and contains(.//span, 'Sign In')]`;
}
