// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row justify="center" class="flex-wrap">
        <v-col v-if="hideFreePlan === false" cols="12" sm="10" md="6" :lg="!pkgAvailable && isUpgradeFlow ? 6 : 4">
            <PricingPlanCard id="free-plan" :disable-cta="!!customFreePlan" :plan="customFreePlan ?? freePlan" @cta-click="emit('freeClick')" />
        </v-col>

        <v-col cols="12" sm="10" md="6" :lg="!pkgAvailable && isUpgradeFlow ? 6 : 4">
            <PricingPlanCard :plan="proPlan" :class="{'pro-border': !pkgAvailable }" @cta-click="emit('proClick')" />
        </v-col>

        <v-col v-if="pkgAvailable && pricingPlan" cols="12" sm="10" md="6" lg="4">
            <PricingPlanCard :plan="pricingPlan" class="pkg-border" @cta-click="emit('pkgClick')" />
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { VCol, VRow } from 'vuetify/components';
import { computed } from 'vue';

import { useBillingStore } from '@/store/modules/billingStore';
import { FREE_PLAN_INFO, PricingPlanInfo } from '@/types/common';

import PricingPlanCard from '@/components/dialogs/upgradeAccountFlow/PricingPlanCard.vue';

const billingStore = useBillingStore();

defineProps<{
    customFreePlan?: PricingPlanInfo;
    hideFreePlan?: boolean;
    isUpgradeFlow?: boolean;
}>();

const emit = defineEmits<{
    freeClick: [];
    proClick: [];
    pkgClick: [];
}>();

const freePlan = FREE_PLAN_INFO;
const proPlan = computed(() => billingStore.proPlanInfo);
const pricingPlan = computed(() => billingStore.state.pricingPlanInfo);
const pkgAvailable = computed<boolean>(() => billingStore.state.pricingPlansAvailable);
</script>

<style scoped lang="scss">
.pro-border {
    border: 2px solid rgb(var(--v-theme-primary)) !important;
}

.pkg-border {
    border: 2px solid rgb(var(--v-theme-secondary)) !important;
}
</style>