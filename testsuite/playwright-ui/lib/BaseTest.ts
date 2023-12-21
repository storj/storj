// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { test as baseTest } from '@playwright/test';
import { LoginPage, LoginPageV2 } from '@pages/LoginPage';
import { DashboardPage } from '@pages/DashboardPage';
import { NavigationMenu } from '@pages/NavigationMenu';
import { BucketsPage } from '@pages/BucketsPage';
import { SignupPage, SignupPageV2 } from '@pages/SignupPage';
import { AllProjectsPage } from '@pages/AllProjectsPage';
import { Common } from '@pages/Common';

const test = baseTest.extend<{
    loginPage: LoginPage;
    loginPageV2: LoginPageV2;
    dashboardPage: DashboardPage;
    navigationMenu: NavigationMenu;
    bucketsPage: BucketsPage;
    signupPage: SignupPage;
    signupPageV2: SignupPageV2;
    allProjectsPage: AllProjectsPage;
    common: Common;
}>({
    loginPage: async ({ page }, use) => {
        await use(new LoginPage(page));
    },
    loginPageV2: async ({ page }, use) => {
        await use(new LoginPageV2(page));
    },
    dashboardPage: async ({ page }, use) => {
        await use(new DashboardPage(page));
    },
    navigationMenu: async ({ page }, use) => {
        await use(new NavigationMenu(page));
    },
    bucketsPage: async ({ page }, use) => {
        await use(new BucketsPage(page));
    },
    signupPage: async ({ page }, use) => {
        await use(new SignupPage(page));
    },
    signupPageV2: async ({ page }, use) => {
        await use(new SignupPageV2(page));
    },
    allProjectsPage: async ({ page }, use) => {
        await use(new AllProjectsPage(page));
    },
    common: async ({ page }, use) => {
        await use(new Common(page));
    },
});

export default test;
