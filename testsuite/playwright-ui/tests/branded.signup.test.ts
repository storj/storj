import test from '@lib/BaseTest';

test.describe('Check for branded signup page, and sign up personal/business accounts', () => {
    test('Verify IX Branding and Signup Personal/Business', async ({signupPage}) => {
        const name = 'John Doe';
        const email = 'test123@test.test';
        const password = 'qazwsx';
        const company = 'Storjing';
        const position = 'CEO';

        await signupPage.navigateToPartnerSignup();
        await signupPage.verifyIXBrandedHeader();
        await signupPage.verifyIXBrandedSubHeader();

        await signupPage.signupApplicationPersonal(name, email, password);
        await signupPage.verifySuccessMessage();

        await signupPage.navigateToPartnerSignup();
        await signupPage.verifyIXBrandedHeader();

        await signupPage.signupApplicationBusiness(name, email, password, company, position);
        await signupPage.verifySuccessMessage();
    })
});
