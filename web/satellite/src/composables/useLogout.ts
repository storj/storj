// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { useRouter } from 'vue-router';

import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AuthHttpApi } from '@/api/auth';
import { ROUTES } from '@/router';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';

export function useLogout() {
    const auth: AuthHttpApi = new AuthHttpApi();
    const router = useRouter();

    const analyticsStore = useAnalyticsStore();
    const bucketsStore = useBucketsStore();
    const appStore = useAppStore();
    const agStore = useAccessGrantsStore();
    const pmStore = useProjectMembersStore();
    const billingStore = useBillingStore();
    const usersStore = useUsersStore();
    const notificationsStore = useNotificationsStore();
    const projectsStore = useProjectsStore();
    const obStore = useObjectBrowserStore();

    async function clearStores(): Promise<void> {
        await Promise.all([
            pmStore.clear(),
            projectsStore.clear(),
            usersStore.clear(),
            agStore.stopWorker(),
            agStore.clear(),
            notificationsStore.clear(),
            bucketsStore.clear(),
            appStore.clear(),
            billingStore.clear(),
            obStore.clear(),
        ]);
    }

    async function logout(): Promise<void> {
        analyticsStore.eventTriggered(AnalyticsEvent.LOGOUT_CLICKED);
        await auth.logout();

        await clearStores();

        await router.push(ROUTES.Login.path);
    }

    return {
        clearStores,
        logout,
    };
}
