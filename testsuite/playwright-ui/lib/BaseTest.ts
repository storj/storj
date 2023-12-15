// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import {test as baseTest} from '@playwright/test';
import {LoginPage} from '@pages/LoginPage';
import {DashboardPage} from '@pages/DashboardPage';
import {NavigationMenu} from '@pages/NavigationMenu';
import {BucketsPage} from '@pages/BucketsPage';
import {SignupPage} from "@pages/SignupPage";
import {AllProjectsPage} from "@pages/AllProjectsPage";

const test = baseTest.extend<{
    loginPage: LoginPage;
    dashboardPage: DashboardPage;
    navigationMenu: NavigationMenu;
    bucketsPage: BucketsPage;
    signupPage: SignupPage;
    allProjectsPage: AllProjectsPage;

}>({
    loginPage: async ({page}, use) => {
        await use(new LoginPage(page));
    },
    dashboardPage: async ({page}, use) => {
        await use(new DashboardPage(page));
    },
    navigationMenu: async ({page}, use) => {
        await use(new NavigationMenu(page));
    },
    bucketsPage: async ({page}, use) => {
        await use(new BucketsPage(page));
    },
    signupPage: async ({page}, use) => {
        await use(new SignupPage(page));
    },
    allProjectsPage: async ({page}, use) => {
        await use(new AllProjectsPage(page));
    }
});

export default test;
