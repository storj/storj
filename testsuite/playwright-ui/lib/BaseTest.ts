// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { test as baseTest } from '@playwright/test';
import { LoginPage, LoginPageV2 } from '@pages/LoginPage';
import { DashboardPage } from '@pages/DashboardPage';
import { NavigationMenu, NavigationMenuV2 } from '@pages/NavigationMenu';
import { BucketsPage, BucketsPageV2 } from '@pages/BucketsPage';
import { SignupPage, SignupPageV2 } from '@pages/SignupPage';
import { AllProjectsPage, AllProjectsPageV2 } from '@pages/AllProjectsPage';
import { Common, CommonV2 } from '@pages/Common';
import { ObjectBrowserPage } from '@pages/ObjectBrowserPage';

const test = baseTest.extend<{
    loginPage: LoginPage;
    loginPageV2: LoginPageV2;
    dashboardPage: DashboardPage;
    navigationMenu: NavigationMenu;
    navigationMenuV2: NavigationMenuV2;
    objectBrowserPage: ObjectBrowserPage;
    bucketsPage: BucketsPage;
    bucketsPageV2: BucketsPageV2;
    signupPage: SignupPage;
    signupPageV2: SignupPageV2;
    allProjectsPage: AllProjectsPage;
    allProjectsPageV2: AllProjectsPageV2;
    common: Common;
    commonV2: CommonV2;
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
    navigationMenuV2: async ({ page }, use) => {
        await use(new NavigationMenuV2(page));
    },
    objectBrowserPage: async ({ page }, use) => {
        await use(new ObjectBrowserPage(page));
    },
    bucketsPage: async ({ page }, use) => {
        await use(new BucketsPage(page));
    },
    bucketsPageV2: async ({ page }, use) => {
        await use(new BucketsPageV2(page));
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
    allProjectsPageV2: async ({ page }, use) => {
        await use(new AllProjectsPageV2(page));
    },
    common: async ({ page }, use) => {
        await use(new Common(page));
    },
    commonV2: async ({ page }, use) => {
        await use(new CommonV2(page));
    },
});

export default test;
