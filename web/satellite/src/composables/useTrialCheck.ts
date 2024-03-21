// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed } from 'vue';

import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';
import { ExpirationInfo, User } from '@/types/users';

export function useTrialCheck() {
    const userStore = useUsersStore();
    const appStore = useAppStore();
    const configStore = useConfigStore();

    const user = computed<User>(() => userStore.state.user);
    const expirationInfo = computed<ExpirationInfo>(() => user.value.getExpirationInfo(configStore.state.config.daysBeforeTrialEndNotification));

    const isTrialExpirationBanner = computed<boolean>(() => {
        if (user.value.paidTier) return false;

        return user.value.freezeStatus.trialExpiredFrozen || expirationInfo.value.isCloseToExpiredTrial;
    });

    const isExpired = computed<boolean>(() => user.value.freezeStatus.trialExpiredFrozen);

    function withTrialCheck(callback: () => void | Promise<void>): void {
        if (!user.value.paidTier && user.value.freezeStatus.trialExpiredFrozen) {
            appStore.toggleExpirationDialog(true);
            return;
        }

        callback();
    }

    return {
        isTrialExpirationBanner,
        isExpired,
        expirationInfo,
        withTrialCheck,
    };
}
