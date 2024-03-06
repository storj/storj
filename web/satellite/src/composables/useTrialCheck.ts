// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed } from 'vue';

import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';
import { User } from '@/types/users';

export function useTrialCheck() {
    const userStore = useUsersStore();
    const appStore = useAppStore();
    const configStore = useConfigStore();

    const user = computed<User>(() => userStore.state.user);

    const isTrialExpirationBanner = computed<boolean>(() => {
        if (user.value.paidTier) return false;

        const expirationInfo = user.value.getExpirationInfo(configStore.state.config.daysBeforeTrialEndNotification);

        return user.value.freezeStatus.trialExpiredFrozen || expirationInfo.isCloseToExpiredTrial;
    });

    const isExpired = computed<boolean>(() => user.value.freezeStatus.trialExpiredFrozen);

    function withTrialCheck(callback: () => void | Promise<void>): void {
        const user = userStore.state.user;
        if (!user.paidTier && user.freezeStatus.trialExpiredFrozen) {
            appStore.toggleExpirationDialog(true);
            return;
        }

        callback();
    }

    return {
        isTrialExpirationBanner,
        isExpired,
        withTrialCheck,
    };
}
