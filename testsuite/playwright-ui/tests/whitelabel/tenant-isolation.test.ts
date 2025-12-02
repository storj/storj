// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import test from '@lib/BaseTest';
import { createTenantContext, createTenantPages } from '@lib/BaseTest';
import { v4 as uuidv4 } from 'uuid';
import { testConfig } from '@config/testConfig';
import { createAndOnboardUser } from '@config/tests/common';

test.describe('Whitelabel Tenant Isolation', () => {
    const sharedEmail = `${uuidv4()}@example.test`;
    const password = 'TestPassword123!';

    const tenant1Hostname = 'tenant1.localhost.test';
    const tenant2Hostname = 'tenant2.localhost.test';

    test('Same email creates separate accounts and prevents cross-tenant access', async ({ browser }) => {
        const tenant1 = await createTenantContext(browser);
        const tenant1Pages = createTenantPages(tenant1);

        const tenant2 = await createTenantContext(browser);
        const tenant2Pages = createTenantPages(tenant2);

        await createAndOnboardUser({
            signupPage: tenant1Pages.signupPage,
            loginPage: tenant1Pages.loginPage,
            navigationMenu: tenant1Pages.navigationMenu,
            email: sharedEmail,
            password: password,
            name: 'User One',
            companyName: 'Company One',
            managedEnc: false,
            dontLogout: true,
            baseURL: `http://${tenant1Hostname}:${testConfig.port}`,
            skipBilling: true,
        });

        const tenant1URL = tenant1.page.url();
        const projectIdMatch = tenant1URL.match(/\/projects\/([^/]+)/);
        const projectId = projectIdMatch ? projectIdMatch[1] : null;

        await createAndOnboardUser({
            signupPage: tenant2Pages.signupPage,
            loginPage: tenant2Pages.loginPage,
            navigationMenu: tenant2Pages.navigationMenu,
            email: sharedEmail,
            password: password,
            name: 'User Two',
            companyName: 'Company Two',
            managedEnc: false,
            dontLogout: true,
            baseURL: `http://${tenant2Hostname}:${testConfig.port}`,
            skipBilling: true,
        });

        if (projectId) {
            await tenant2.page.goto(`http://${tenant1Hostname}:${testConfig.port}/projects/${projectId}`);
            await tenant2.page.waitForURL(/projects\/[^/]+/, { timeout: 5000 });
            const currentURL = tenant2.page.url();
            test.expect(currentURL).not.toContain(projectId);
        }
    });

    test('Login with tenant context maintains isolation', async ({ browser }) => {
        const email1 = `${uuidv4()}@example.test`;
        const email2 = `${uuidv4()}@example.test`;

        const tenant1 = await createTenantContext(browser);
        const tenant2 = await createTenantContext(browser);
        const tenant1Pages = createTenantPages(tenant1);
        const tenant2Pages = createTenantPages(tenant2);

        await tenant1.page.goto(`http://${tenant1Hostname}:${testConfig.port}/signup`);
        await tenant1Pages.signupPage.signupFirstStep(email1, password);
        await tenant1Pages.signupPage.verifySuccessMessage();

        await tenant2.page.goto(`http://${tenant2Hostname}:${testConfig.port}/signup`);
        await tenant2Pages.signupPage.signupFirstStep(email2, password);
        await tenant2Pages.signupPage.verifySuccessMessage();

        await tenant1Pages.signupPage.navigateToLogin();
        await tenant1Pages.loginPage.loginByCreds(email1, password);
        await tenant1Pages.loginPage.verifySetupAccountFirstStep();

        await tenant2Pages.signupPage.navigateToLogin();
        await tenant2Pages.loginPage.loginByCreds(email1, password);
        await tenant2Pages.loginPage.verifyInvalidCredentials();
    });

    test('Branding is correct for a tenant', async ({ browser }) => {
        const expectedSupportUrl = 'https://support.tenant1.example';

        const tenant = await createTenantContext(browser);

        const email1 = `${uuidv4()}@example.test`;
        await createAndOnboardUser({
            signupPage: createTenantPages(tenant).signupPage,
            loginPage: createTenantPages(tenant).loginPage,
            navigationMenu: createTenantPages(tenant).navigationMenu,
            email: email1,
            password: password,
            name: 'User One',
            companyName: 'Company One',
            managedEnc: false,
            dontLogout: true,
            baseURL: `http://${tenant1Hostname}:${testConfig.port}`,
            skipBilling: true,
        });

        const tenant1Pages = createTenantPages(tenant);
        await tenant1Pages.navigationMenu.openResources();
        const supportLink = tenant.page.locator(`a[href="${expectedSupportUrl}"]`);
        await test.expect(supportLink).toBeVisible();

        await tenant1Pages.navigationMenu.openAccountSettings();
        const upgradeButton = tenant.page.locator('div:has-text(" Upgrade ")');
        await test.expect(upgradeButton).not.toBeVisible();
    });
});
