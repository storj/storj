// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed } from 'vue';

import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';
import { ExpirationInfo, User } from '@/types/users';
import { useProjectsStore } from '@/store/modules/projectsStore';

export function usePreCheck() {
    const userStore = useUsersStore();
    const projectsStore = useProjectsStore();
    const appStore = useAppStore();
    const configStore = useConfigStore();

    const user = computed<User>(() => userStore.state.user);
    const isUserProjectOwner = computed<boolean>(() => projectsStore.state.selectedProject.ownerId === user.value.id);
    const expirationInfo = computed<ExpirationInfo>(() => user.value.getExpirationInfo(configStore.state.config.daysBeforeTrialEndNotification));

    const isTrialExpirationBanner = computed<boolean>(() => {
        if (user.value.hasPaidPrivileges || !configStore.billingEnabled) return false;

        return user.value.freezeStatus.trialExpiredFrozen || expirationInfo.value.isCloseToExpiredTrial;
    });

    const isExpired = computed<boolean>(() => user.value.freezeStatus.trialExpiredFrozen);

    function withTrialCheck(callback: () => void | Promise<void>, skipProjectOwningCheck = false): void {
        if (configStore.billingEnabled) {
            const isTrialExpired = !user.value.hasPaidPrivileges && user.value.freezeStatus.trialExpiredFrozen;
            const isEligibleForExpirationDialog = isTrialExpired && (skipProjectOwningCheck || isUserProjectOwner.value);

            if (isEligibleForExpirationDialog) {
                appStore.toggleExpirationDialog(true);
                return;
            }
        }

        callback();
    }

    function withManagedPassphraseCheck(callback: () => void | Promise<void>): void {
        if (appStore.state.managedPassphraseNotRetrievable) {
            appStore.toggleManagedPassphraseErrorDialog(true);
            return;
        }

        callback();
    }

    return {
        isTrialExpirationBanner,
        isExpired,
        isUserProjectOwner,
        expirationInfo,
        withTrialCheck,
        withManagedPassphraseCheck,
    };
}
