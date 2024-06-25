// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row :align="smAndDown ? 'center' : 'start'" :justify="smAndDown ? 'start' : 'space-between'" :class="{'flex-column': smAndDown}">
        <v-col v-for="(plan, index) in plans" :key="index" :cols="smAndDown ? 10 : 6" class="select-item">
            <PricingPlanContainer
                :plan="plan"
                @select="(p) => emit('select', p)"
            />
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { onBeforeMount, ref } from 'vue';
import { VCol, VRow } from 'vuetify/components';
import { useDisplay } from 'vuetify';

import { PricingPlanInfo, PricingPlanType } from '@/types/common';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';

import PricingPlanContainer from '@/components/billing/pricingPlans/PricingPlanContainer.vue';

const billingStore = useBillingStore();
const usersStore = useUsersStore();
const notify = useNotify();
const { smAndDown } = useDisplay();

const props = withDefaults(defineProps<{
    // the upgrade dialog for example will not show the free plan.
    showFreePlan?: boolean;
}>(), {
    showFreePlan: false,
});

const emit = defineEmits<{
    select: [PricingPlanInfo];
}>();

const plans = ref<PricingPlanInfo[]>([
    new PricingPlanInfo(
        PricingPlanType.PRO,
        'Pro Account',
        '',
        'Only pay for what you need. $4/TB stored per month* $7/TB for download bandwidth.',
        '*Additional per-segment fee of $0.0000088 applies.',
        null,
        null,
        'Add a credit card to activate your Pro Account.<br><br>Only pay for what you use.',
        'No charge today',
        '',
    ),
]);

/**
 * Loads pricing plan config. Assumes that user is already eligible for a plan prior to component being mounted.
 */
onBeforeMount(async () => {
    if (props.showFreePlan) {
        plans.value = [
            ...plans.value,
            new PricingPlanInfo(
                PricingPlanType.FREE,
                'Free Trial',
                'Limited',
                'Free usage up to 25GB storage and 25GB egress bandwidth for 30 days.',
                null,
                null,
                null,
                'Start for free to try Storj and upgrade later.',
                null,
                'Limited 25',
            ),
        ];
    }

    const plan = billingStore.state.pricingPlanInfo;
    if (!plan) {
        notify.error(`No pricing plan configuration for partner '${usersStore.state.user.partner}'.`, null);
        return;
    }
    plan.type = PricingPlanType.PARTNER;
    plans.value.unshift(plan);
});
</script>

<style scoped lang="scss">
.select-item {
    height: 450px;
}
</style>
