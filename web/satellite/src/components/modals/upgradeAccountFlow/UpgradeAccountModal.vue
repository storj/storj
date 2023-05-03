// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <UpgradeInfoStep
                v-if="step === UpgradeAccountStep.Info"
                :on-upgrade="() => setStep(UpgradeAccountStep.Options)"
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
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { useAppStore } from '@/store/modules/appStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';

import VModal from '@/components/common/VModal.vue';
import UpgradeInfoStep from '@/components/modals/upgradeAccountFlow/UpgradeInfoStep.vue';
import UpgradeOptionsStep from '@/components/modals/upgradeAccountFlow/UpgradeOptionsStep.vue';
import AddCreditCardStep from '@/components/modals/upgradeAccountFlow/AddCreditCardStep.vue';
import SuccessStep from '@/components/modals/upgradeAccountFlow/SuccessStep.vue';
import AddTokensStep from '@/components/modals/upgradeAccountFlow/AddTokensStep.vue';

enum UpgradeAccountStep {
    Info = 'infoStep',
    Options = 'optionsStep',
    AddCC = 'addCCStep',
    AddTokens = 'addTokensStep',
    Success = 'successStep',
}

const appStore = useAppStore();
const billingStore = useBillingStore();
const notify = useNotify();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

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

        analytics.eventTriggered(AnalyticsEvent.ADD_FUNDS_CLICKED);

        setStep(UpgradeAccountStep.AddTokens);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
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
 * Closes upgrade account modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}
</script>
