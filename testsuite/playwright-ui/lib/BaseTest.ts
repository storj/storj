// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { test as baseTest } from '@playwright/test';
import { LoginPage } from '@pages/LoginPage';
import { NavigationMenu } from '@pages/NavigationMenu';
import { BucketsPage } from '@pages/BucketsPage';
import { SignupPage } from '@pages/SignupPage';
import { AllProjectsPage } from '@pages/AllProjectsPage';
import { ObjectBrowserPage } from '@pages/ObjectBrowserPage';

const test = baseTest.extend<{
    loginPage: LoginPage;
    navigationMenu: NavigationMenu;
    objectBrowserPage: ObjectBrowserPage;
    bucketsPage: BucketsPage;
    signupPage: SignupPage;
    allProjectsPage: AllProjectsPage;
}>({
    loginPage: async ({ page }, use) => {
        await use(new LoginPage(page));
    },
    navigationMenu: async ({ page }, use) => {
        await use(new NavigationMenu(page));
    },
    objectBrowserPage: async ({ page }, use) => {
        await use(new ObjectBrowserPage(page));
    },
    bucketsPage: async ({ page }, use) => {
        await use(new BucketsPage(page));
    },
    signupPage: async ({ page }, use) => {
        await use(new SignupPage(page));
    },
    allProjectsPage: async ({ page }, use) => {
        await use(new AllProjectsPage(page));
    },
});

export default test;
