export class SignupPageObjects {
    // SIGNUP
    protected static INPUT_NAME_XPATH = `//input[@id='Full Name']`;
    protected static INPUT_EMAIL_XPATH = `//input[@id='Email Address']`;
    protected static INPUT_PASSWORD_XPATH = `//input[@id='Password']`;
    protected static INPUT_RETYPE_PASSWORD_XPATH = `//input[@id='Retype Password']`;
    protected static TOS_CHECKMARK_BUTTON_XPATH = `.checkmark-container`;

    // SIGNUP SUCCESS PAGE
    protected static SIGNUP_SUCCESS_MESSAGE_XPATH = `//h2[contains(text(),"You\'re almost there!")]`;
    protected static GOTO_LOGIN_PAGE_BUTTON_XPATH = `//a[contains(text(),'Go to Login page')]`;

    // IX BRANDED SIGNUP
    protected static CREATE_ACCOUNT_BUTTON_XPATH = '//span[contains(text(),\'Create an iX-Storj Account\')]';
    protected static IX_BRANDED_HEADER_TEXT_XPATH = '//h1[contains(text(),\'Globally Distributed Storage for TrueNAS\')]';
    protected static IX_BRANDED_SUBHEADER_TEXT_XPATH = '//p[contains(text(),\'iX and Storj have partnered to offer a secure, hig\')]';

    // BUSINESS TAB
    protected static IX_BRANDED_BUSINESS_BUTTON_XPATH = `//li[contains(text(),'Business')]`;
    protected static COMPANY_NAME_INPUT_XPATH = `//input[@id='Company Name']`;
    protected static POSITION_INPUT_XPATH = `//input[@id='Position']`;

}
