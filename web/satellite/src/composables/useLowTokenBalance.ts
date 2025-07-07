// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed } from 'vue';

import { microDollarsToCents } from '@/utils/strings';
import { useBillingStore } from '@/store/modules/billingStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';

export function useLowTokenBalance() {
    const userStore = useUsersStore();
    const configStore = useConfigStore();
    const billingStore = useBillingStore();

    return computed<boolean>(() => {
        const notEnoughBalance = billingStore.state.nativePaymentsHistory.length > 0 &&
            billingStore.state.balance.sum < microDollarsToCents(configStore.state.config.userBalanceForUpgrade);

        return (
            userStore.state.user.isPaid && !billingStore.state.creditCards.length && notEnoughBalance
        ) || (
            billingStore.state.creditCards.length > 0 && billingStore.state.balance.sum > 0 && notEnoughBalance
        );
    });
}
