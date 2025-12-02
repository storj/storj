// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { SignupPage } from '@pages/SignupPage';
import { LoginPage } from '@pages/LoginPage';
import { NavigationMenu } from '@pages/NavigationMenu';

export type CreateAndOnboardUserParams = {
    signupPage: SignupPage;
    loginPage: LoginPage;
    navigationMenu: NavigationMenu;
    email: string;
    password: string;
    name: string;
    companyName: string;
    managedEnc: boolean;
    dontLogout?: boolean;
    baseURL?: string;
    skipBilling?: boolean;
}

export async function createAndOnboardUser(params: CreateAndOnboardUserParams): Promise<void> {
    if (params.baseURL) {
        await params.signupPage.page.goto(`${params.baseURL}/signup`);
    } else {
        await params.signupPage.navigateToSignup();
    }
    await params.signupPage.signupFirstStep(params.email, params.password);
    await params.signupPage.verifySuccessMessage();
    await params.signupPage.navigateToLogin();

    await params.loginPage.loginByCreds(params.email, params.password);
    await params.loginPage.verifySetupAccountFirstStep();
    await params.loginPage.fillSetupForm(params.name, params.companyName);
    if (!params.skipBilling) {
        await params.loginPage.selectFreeTrial();
    }
    await params.loginPage.selectManagedEnc(params.managedEnc);
    await params.loginPage.ensureSetupSuccess();
    await params.loginPage.finishSetup();

    if (!params.dontLogout) {
        await params.navigationMenu.logout();
    }
}
