// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <PricingPlans
        :loading="loading"
        is-upgrade-flow
        :custom-free-plan="isMemberUpgrade ? undefined : freePlan"
        :hide-free-plan="!isMemberUpgrade && smAndDown"
        @free-click="isMemberUpgrade ? emit('startFreeTrial') : () => {}"
        @pro-click="emit('upgrade', proPlan)"
        @pkg-click="pricingPlan ? emit('upgrade', pricingPlan) : null"
    />
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useDisplay } from 'vuetify';

import { usePreCheck } from '@/composables/usePreCheck';
import { FREE_PLAN_INFO, PricingPlanInfo } from '@/types/common';
import { useBillingStore } from '@/store/modules/billingStore';

import PricingPlans from '@/components/dialogs/upgradeAccountFlow/PricingPlans.vue';

const billingStore = useBillingStore();
const { smAndDown } = useDisplay();
const { isExpired, expirationInfo } = usePreCheck();

defineProps<{
    loading: boolean;
    isMemberUpgrade: boolean;
}>();

const emit = defineEmits<{
    upgrade: [PricingPlanInfo];
    startFreeTrial: [];
}>();

const proPlan = computed(() => billingStore.proPlanInfo);
const pricingPlan = computed(() => billingStore.state.pricingPlanInfo);

/**
 * Returns free trial button label based on expiration status.
 */
const freeTrialButtonLabel = computed<string>(() => {
    if (isExpired.value) return 'Trial Expired';

    if (!expirationInfo.value.days) {
        return 'Trial Expiring soon';
    }

    return `${expirationInfo.value.days} day${expirationInfo.value.days !== 1 ? 's' : ''} remaining`;
});

const freePlan = computed(() => {
    const plan = { ...FREE_PLAN_INFO };
    plan.planCTA = freeTrialButtonLabel.value;
    return plan;
});
</script>
