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

import { RouteConfig } from '@/router';
import { PricingPlanInfo, PricingPlanType } from '@/types/common';
import { User } from '@/types/users';
import { useNotify, useRouter, useStore } from '@/utils/hooks';
import { MetaUtils } from '@/utils/meta';
import { PaymentsHttpApi } from '@/api/payments';

import PricingPlanContainer from '@/components/onboardingTour/steps/pricingPlanFlow/PricingPlanContainer.vue';
import VLoader from '@/components/common/VLoader.vue';

const store = useStore();
const router = useRouter();
const notify = useNotify();
const payments: PaymentsHttpApi = new PaymentsHttpApi();

const isLoading = ref<boolean>(true);

const plans = ref<PricingPlanInfo[]>([
    new PricingPlanInfo(
        PricingPlanType.PRO,
        'Pro Account',
        '150 GB Free',
        'Only pay for what you need at $4/TB storage per month and $7/TB download bandwidth per month.',
        null,
        null,
        null,
        'Just add a card to activate Pro Account',
        '150GB Free',
    ),
    new PricingPlanInfo(
        PricingPlanType.FREE,
        'Starter Package',
        'Limited',
        'Free usage of 150GB storage and 150GB download bandwidth per month.',
        null,
        null,
        null,
        'Start for free to try Storj and upgrade later.',
        'Limited 150',
    ),
]);

/*
 * Loads pricing plan config.
 */
onBeforeMount(async () => {
    const user: User = store.getters.user;
    const nextPath = RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path;

    const pricingPkgsEnabled = Boolean(MetaUtils.getMetaContent('pricing-packages-enabled'));
    if (!pricingPkgsEnabled || user.paidTier || !user.partner) {
        router.push(nextPath);
        return;
    }

    let pkgAvailable = false;
    try {
        pkgAvailable = await payments.pricingPackageAvailable();
    } catch (error) {
        notify.error(error.message, null);
    }
    if (!pkgAvailable) {
        router.push(nextPath);
        return;
    }

    let config;
    try {
        config = require('@/components/onboardingTour/steps/pricingPlanFlow/pricingPlanConfig.json');
    } catch {
        notify.error('No pricing plan configuration file.', null);
        router.push(nextPath);
        return;
    }

    const plan = config[user.partner];
    if (!plan) {
        notify.error(`No pricing plan configuration for partner '${user.partner}'.`, null);
        router.push(nextPath);
        return;
    }

    plans.value.unshift(new PricingPlanInfo(
        PricingPlanType.PARTNER,
        plan.title,
        plan.containerSubtitle,
        plan.containerDescription,
        plan.price,
        plan.oldPrice,
        plan.activationSubtitle,
        plan.activationDescription,
        plan.successSubtitle,
    ));

    isLoading.value = false;
});
</script>

<style scoped lang="scss">
.pricing-area {

    &__loader {
        position: fixed;
        top: 0;
        bottom: 0;
        left: 0;
        right: 0;
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

@media screen and (max-width: 963px) {

    .pricing-area__plans {
        max-width: 444px;
        flex-direction: column;
    }
}
</style>
