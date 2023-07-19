// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pricing-area">
        <VLoader v-if="isLoading" class="pricing-area__loader" width="90px" height="90px" />
        <template v-else>
            <h1 class="pricing-area__title" aria-roledescription="title">Welcome to Storj</h1>
            <p class="pricing-area__subtitle">Select an account type to continue.</p>
            <div class="pricing-area__plans">
                <PricingPlanContainer
                    v-for="(plan, index) in plans"
                    :key="index"
                    :plan="plan"
                />
            </div>
        </template>
    </div>
</template>

<script setup lang="ts">
import { onBeforeMount, ref } from 'vue';
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { PricingPlanInfo, PricingPlanType } from '@/types/common';
import { User } from '@/types/users';
import { useNotify } from '@/utils/hooks';
import { PaymentsHttpApi } from '@/api/payments';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';

import PricingPlanContainer from '@/components/account/billing/pricingPlans/PricingPlanContainer.vue';
import VLoader from '@/components/common/VLoader.vue';

const configStore = useConfigStore();
const usersStore = useUsersStore();
const router = useRouter();
const notify = useNotify();
const payments: PaymentsHttpApi = new PaymentsHttpApi();

const isLoading = ref<boolean>(true);

const plans = ref<PricingPlanInfo[]>([
    new PricingPlanInfo(
        PricingPlanType.PRO,
        'Pro Account',
        '25 GB Free',
        'Only pay for what you need. $4/TB stored per month* $7/TB for egress bandwidth.',
        '*Additional per-segment fee of $0.0000088 applies.',
        null,
        null,
        'Add a credit card to activate your Pro Account.<br><br>Get 25GB free storage and egress. Only pay for what you use beyond that.',
        'No charge today',
        '25GB Free',
    ),
    new PricingPlanInfo(
        PricingPlanType.FREE,
        'Free Account',
        'Limited',
        'Free usage up to 25GB storage and 25GB egress bandwidth per month.',
        null,
        null,
        null,
        'Start for free to try Storj and upgrade later.',
        null,
        'Limited 25',
    ),
]);

/*
 * Loads pricing plan config.
 */
onBeforeMount(async () => {
    const user: User = usersStore.state.user;
    let nextPath = RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path;
    if (configStore.state.config.allProjectsDashboard) {
        nextPath = RouteConfig.AllProjectsDashboard.path;
    }

    const pricingPkgsEnabled = configStore.state.config.pricingPackagesEnabled;
    if (!pricingPkgsEnabled || user.paidTier || !user.partner) {
        router.push(nextPath);
        return;
    }

    let pkgAvailable = false;
    try {
        pkgAvailable = await payments.pricingPackageAvailable();
    } catch (error) {
        notify.notifyError(error, null);
    }
    if (!pkgAvailable) {
        router.push(nextPath);
        return;
    }

    let config;
    try {
        config = require('@/components/account/billing/pricingPlans/pricingPlanConfig.json');
    } catch {
        notify.error('No pricing plan configuration file.', null);
        router.push(nextPath);
        return;
    }

    const plan = config[user.partner] as PricingPlanInfo;
    if (!plan) {
        notify.error(`No pricing plan configuration for partner '${user.partner}'.`, null);
        router.push(nextPath);
        return;
    }
    plan.type = PricingPlanType.PARTNER;
    plans.value.unshift(plan);

    if (!usersStore.state.settings.onboardingStart) {
        try {
            await usersStore.updateSettings({ onboardingStart: true });
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.PRICING_PLAN_STEP);
        }
    }

    isLoading.value = false;
});
</script>

<style scoped lang="scss">
.pricing-area {

    &__loader {
        position: fixed;
        inset: 0;
        align-items: center;
    }

    &__title {
        color: #14142b;
        font-family: 'font_bold', sans-serif;
        font-size: 32px;
        line-height: 39px;
        text-align: center;
    }

    &__subtitle {
        margin-top: 12.5px;
        color: #354049;
        font-family: 'font_regular', sans-serif;
        font-weight: 400;
        font-size: 16px;
        line-height: 134.09%;
        text-align: center;
    }

    &__plans {
        margin-top: 41px;
        display: flex;
        gap: 30px;
    }
}

@media screen and (width <= 963px) {

    .pricing-area__plans {
        max-width: 444px;
        flex-direction: column;
    }
}
</style>
