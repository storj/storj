// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <UpgradeInfoStep
                v-if="step === UpgradeAccountStep.Info"
                :on-upgrade="setSecondStep"
                :loading="loading"
            />
            <UpgradeOptionsStep
                v-if="step === UpgradeAccountStep.Options"
                :on-add-card="() => setStep(UpgradeAccountStep.AddCC)"
                :on-add-tokens="onAddTokens"
                :loading="loading"
            />
            <AddCreditCardStep
                v-if="step === UpgradeAccountStep.AddCC"
                :set-success="() => setStep(UpgradeAccountStep.Success)"
            />
            <AddTokensStep v-if="step === UpgradeAccountStep.AddTokens" />
            <SuccessStep
                v-if="step === UpgradeAccountStep.Success"
                :on-continue="closeModal"
            />
            <PricingPlanStep
                v-if="step === UpgradeAccountStep.PricingPlan"
            />
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@/store/modules/appStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useNotify } from '@/utils/hooks';
import { PaymentsHttpApi } from '@/api/payments';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { User } from '@/types/users';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VModal from '@/components/common/VModal.vue';
import UpgradeInfoStep from '@/components/modals/upgradeAccountFlow/UpgradeInfoStep.vue';
import UpgradeOptionsStep from '@/components/modals/upgradeAccountFlow/UpgradeOptionsStep.vue';
import AddCreditCardStep from '@/components/modals/upgradeAccountFlow/AddCreditCardStep.vue';
import SuccessStep from '@/components/modals/upgradeAccountFlow/SuccessStep.vue';
import AddTokensStep from '@/components/modals/upgradeAccountFlow/AddTokensStep.vue';
import PricingPlanStep from '@/components/modals/upgradeAccountFlow/PricingPlanStep.vue';

enum UpgradeAccountStep {
    Info = 'infoStep',
    Options = 'optionsStep',
    AddCC = 'addCCStep',
    AddTokens = 'addTokensStep',
    Success = 'successStep',
    PricingPlan = 'pricingPlanStep',
}

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
const billingStore = useBillingStore();
const notify = useNotify();
const payments: PaymentsHttpApi = new PaymentsHttpApi();

const step = ref<UpgradeAccountStep>(UpgradeAccountStep.Info);
const loading = ref<boolean>(false);

/**
 * Claims wallet and sets add token step.
 */
async function onAddTokens(): Promise<void> {
    if (loading.value) return;

    loading.value = true;

    try {
        await billingStore.claimWallet();

        analyticsStore.eventTriggered(AnalyticsEvent.ADD_FUNDS_CLICKED);

        setStep(UpgradeAccountStep.AddTokens);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
    }

    loading.value = false;
}

/**
 * Sets specific flow step.
 */
function setStep(s: UpgradeAccountStep) {
    step.value = s;
}

/**
 * Sets second step in the flow (after user clicks to upgrade).
 * Most users will go to the Options step, but if a user is eligible for a
 * pricing plan (and pricing plans are enabled), they will be sent to the PricingPlan step.
 */
async function setSecondStep() {
    if (loading.value) return;

    loading.value = true;

    const user: User = usersStore.state.user;
    const pricingPkgsEnabled = configStore.state.config.pricingPackagesEnabled;
    if (!pricingPkgsEnabled || !user.partner) {
        setStep(UpgradeAccountStep.Options);
        loading.value = false;
        return;
    }

    let pkgAvailable = false;
    try {
        pkgAvailable = await payments.pricingPackageAvailable();
    } catch (error) {
        notify.notifyError(error, null);
        setStep(UpgradeAccountStep.Options);
        loading.value = false;
        return;
    }
    if (!pkgAvailable) {
        setStep(UpgradeAccountStep.Options);
        loading.value = false;
        return;
    }

    setStep(UpgradeAccountStep.PricingPlan);

    loading.value = false;
}

/**
 * Closes upgrade account modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}
</script>
