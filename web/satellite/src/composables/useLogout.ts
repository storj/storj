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
import { useConfigStore } from '@/store/modules/configStore';
import { useAccessGrantWorker } from '@/composables/useAccessGrantWorker';
import { useRestApiKeysStore } from '@/store/modules/apiKeysStore';
import { useDomainsStore } from '@/store/modules/domainsStore';
import { useComputeStore } from '@/store/modules/computeStore';

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
    const configStore = useConfigStore();
    const apiKeysStore = useRestApiKeysStore();
    const domainsStore = useDomainsStore();
    const computeStore = useComputeStore();

    async function clearStores(): Promise<void> {
        const { stop } = useAccessGrantWorker();

        stop();

        await Promise.all([
            pmStore.clear(),
            projectsStore.clear(),
            usersStore.clear(),
            agStore.clear(),
            notificationsStore.clear(),
            bucketsStore.clear(),
            appStore.clear(),
            billingStore.clear(),
            obStore.clear(),
            apiKeysStore.clear(),
            domainsStore.clear(),
            computeStore.clear(),
        ]);
    }

    async function logout(): Promise<void> {
        analyticsStore.eventTriggered(AnalyticsEvent.LOGOUT_CLICKED);
        await auth.logout(configStore.state.config.csrfToken);

        await clearStores();

        await router.push(ROUTES.Login.path);
    }

    return {
        clearStores,
        logout,
    };
}
