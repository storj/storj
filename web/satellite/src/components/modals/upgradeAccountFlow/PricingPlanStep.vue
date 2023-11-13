// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <UpgradeAccountWrapper title="Upgrade">
        <template #content>
            <div class="pricing-area">
                <VLoader v-if="isLoading" class="pricing-area__loader" width="90px" height="90px" />
                <template v-else>
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
    </UpgradeAccountWrapper>
</template>

<script setup lang="ts">
import { onBeforeMount, ref } from 'vue';
import { useRouter } from 'vue-router';

import { PricingPlanInfo, PricingPlanType } from '@/types/common';
import { User } from '@/types/users';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';

import UpgradeAccountWrapper from '@/components/modals/upgradeAccountFlow/UpgradeAccountWrapper.vue';
import PricingPlanContainer from '@/components/account/billing/pricingPlans/PricingPlanContainer.vue';
import VLoader from '@/components/common/VLoader.vue';

const usersStore = useUsersStore();
const router = useRouter();
const notify = useNotify();

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
]);

/*
 * Loads pricing plan config. Assumes that user is already eligible for a plan prior to component being mounted.
 */
onBeforeMount(async () => {
    const user: User = usersStore.state.user;

    let config;
    try {
        config = require('@/components/account/billing/pricingPlans/pricingPlanConfig.json');
    } catch {
        notify.error('No pricing plan configuration file.', null);
        return;
    }

    const plan = config[user.partner] as PricingPlanInfo;
    if (!plan) {
        notify.error(`No pricing plan configuration for partner '${user.partner}'.`, null);
        return;
    }
    plan.type = PricingPlanType.PARTNER;
    plans.value.unshift(plan);

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
