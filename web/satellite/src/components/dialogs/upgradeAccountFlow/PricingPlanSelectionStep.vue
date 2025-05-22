// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row :align="mdAndDown ? 'center' : 'start'" :justify="mdAndDown ? 'start' : 'space-between'" :class="{'flex-column': mdAndDown}" class="mt-1 mb-2">
        <v-col v-for="(plan, index) in plans" :key="index" class="select-item">
            <PricingPlanContainer
                :plan="plan"
                @select="(p) => emit('select', p)"
            />
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { computed, onBeforeMount } from 'vue';
import { VCol, VRow } from 'vuetify/components';
import { useDisplay } from 'vuetify';

import { FREE_PLAN_INFO, PricingPlanInfo, PricingPlanType } from '@/types/common';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';

import PricingPlanContainer from '@/components/billing/pricingPlans/PricingPlanContainer.vue';

const billingStore = useBillingStore();
const usersStore = useUsersStore();
const notify = useNotify();
const { mdAndDown } = useDisplay();

const props = withDefaults(defineProps<{
    // the upgrade dialog for example will not show the free plan.
    showFreePlan?: boolean;
}>(), {
    showFreePlan: false,
});

const emit = defineEmits<{
    select: [PricingPlanInfo];
}>();

const plans = computed<PricingPlanInfo[]>(() => {
    const plans = [
        billingStore.proPlanInfo,
    ];
    if (props.showFreePlan) plans.push(FREE_PLAN_INFO);

    const plan = billingStore.state.pricingPlanInfo;
    if (!plan) {
        return plans;
    }
    plan.type = PricingPlanType.PARTNER;
    plans.unshift(plan);

    return plans;
});

/**
 * Loads pricing plan config. Assumes that user is already eligible for a plan prior to component being mounted.
 */
onBeforeMount(async () => {
    if (!billingStore.state.pricingPlanInfo)
        notify.error(`No pricing plan configuration for partner '${usersStore.state.user.partner}'.`, null);
});
</script>
