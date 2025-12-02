// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { test as baseTest, Browser, BrowserContext, Page } from '@playwright/test';
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

export type Pages = {
    loginPage: LoginPage;
    signupPage: SignupPage;
    navigationMenu: NavigationMenu;
    bucketsPage: BucketsPage;
    objectBrowserPage: ObjectBrowserPage;
    allProjectsPage: AllProjectsPage;
    accountSettingsPage: AccountSettingsPage;
    projectDashboardPage: ProjectDashboardPage;
    accessKeysPage: AccessKeysPage;
    teamPage: TeamPage;
};

/**
 * TenantContext represents a browser context configured for a specific tenant.
 * It includes the context itself and a page within that context.
 */
export type TenantContext = {
    context: BrowserContext;
    page: Page;
};

/**
 * Creates a new browser context for a specific tenant.
 * Uses Chrome's --host-resolver-rules to map tenant hostnames to localhost.
 * @param browser - The browser instance
 * @returns TenantContext with the context and a page
 */
export async function createTenantContext(browser: Browser): Promise<TenantContext> {
    const context = await browser.newContext();
    const page = await context.newPage();
    return { context, page };
}

/**
 * Creates all page objects for a given tenant context.
 * @param tenantContext - The tenant context containing the page
 * @returns TenantPages with all page objects initialized
 */
export function createTenantPages(tenantContext: TenantContext): Pages {
    const { page } = tenantContext;
    return {
        loginPage: new LoginPage(page),
        signupPage: new SignupPage(page),
        navigationMenu: new NavigationMenu(page),
        bucketsPage: new BucketsPage(page),
        objectBrowserPage: new ObjectBrowserPage(page),
        allProjectsPage: new AllProjectsPage(page),
        accountSettingsPage: new AccountSettingsPage(page),
        projectDashboardPage: new ProjectDashboardPage(page),
        accessKeysPage: new AccessKeysPage(page),
        teamPage: new TeamPage(page),
    };
}

const test = baseTest.extend<Pages>({
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
