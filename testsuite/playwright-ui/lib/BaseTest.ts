// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { test as baseTest } from '@playwright/test';
import { LoginPage } from '@pages/LoginPage';
import { NavigationMenu } from '@pages/NavigationMenu';
import { BucketsPage } from '@pages/BucketsPage';
import { SignupPage } from '@pages/SignupPage';
import { AllProjectsPage } from '@pages/AllProjectsPage';
import { ObjectBrowserPage } from '@pages/ObjectBrowserPage';
import { AccountSettingsPage } from '@pages/AccountSettingsPage';
import { ProjectDashboardPage } from '@pages/ProjectDashboardPage';
import { AccessKeysPage } from '@pages/AccessKeysPage';
import { TeamPage } from '@pages/TeamPage';

const test = baseTest.extend<{
    loginPage: LoginPage;
    navigationMenu: NavigationMenu;
    objectBrowserPage: ObjectBrowserPage;
    bucketsPage: BucketsPage;
    signupPage: SignupPage;
    allProjectsPage: AllProjectsPage;
    accountSettingsPage: AccountSettingsPage;
    projectDashboardPage: ProjectDashboardPage;
    accessKeysPage: AccessKeysPage;
    teamPage: TeamPage;
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
    accountSettingsPage: async ({ page }, use) => {
        await use(new AccountSettingsPage(page));
    },
    projectDashboardPage: async ({ page }, use) => {
        await use(new ProjectDashboardPage(page));
    },
    accessKeysPage: async ({ page }, use) => {
        await use(new AccessKeysPage(page));
    },
    teamPage: async ({ page }, use) => {
        await use(new TeamPage(page));
    },
});

export default test;
