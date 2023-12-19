// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        scrollable
        min-width="460px"
        :max-width="step === UpgradeAccountStep.Info || step === UpgradeAccountStep.PricingPlanSelection ? '720px' : '460px'"
        transition="fade-transition"
        :persistent="loading"
        :scrim="scrim"
    >
        <v-card ref="content" rounded="xlg">
            <v-card-item class="pa-5 pl-7">
                <template v-if="step === UpgradeAccountStep.Success" #prepend>
                    <img class="d-block" src="@/../static/images/modals/upgradeFlow/success.svg" alt="success">
                </template>
                <v-card-title class="font-weight-bold">{{ stepTitles[step] }}</v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item class="py-4">
                <v-window v-model="step">
                    <v-window-item :value="UpgradeAccountStep.Info">
                        <UpgradeInfoStep
                            :loading="loading"
                            @upgrade="setSecondStep"
                        />
                    </v-window-item>

                    <v-window-item :value="UpgradeAccountStep.Options">
                        <UpgradeOptionsStep
                            :loading="loading"
                            @add-card="() => setStep(UpgradeAccountStep.AddCC)"
                            @add-tokens="onAddTokens"
                        />
                    </v-window-item>

                    <v-window-item :value="UpgradeAccountStep.AddCC">
                        <AddCreditCardStep
                            @success="() => setStep(UpgradeAccountStep.Success)"
                            @back="() => setStep(UpgradeAccountStep.Options)"
                        />
                    </v-window-item>

                    <v-window-item :value="UpgradeAccountStep.AddTokens">
                        <AddTokensStep
                            @back="() => setStep(UpgradeAccountStep.Options)"
                        />
                    </v-window-item>

                    <v-window-item :value="UpgradeAccountStep.Success">
                        <SuccessStep @continue="model = false" />
                    </v-window-item>

                    <v-window-item :value="UpgradeAccountStep.PricingPlanSelection">
                        <PricingPlanSelectionStep @select="onSelectPricingPlan" />
                    </v-window-item>

                    <v-window-item :value="UpgradeAccountStep.PricingPlan">
                        <PricingPlanStep :plan="plan" @close="model = false" @back="setStep(UpgradeAccountStep.PricingPlanSelection)" />
                    </v-window-item>
                </v-window>
            </v-card-item>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VBtn, VCard, VCardItem, VCardTitle, VDialog, VDivider, VWindow, VWindowItem } from 'vuetify/components';

import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@poc/store/appStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useNotify } from '@/utils/hooks';
import { PaymentsHttpApi } from '@/api/payments';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { User } from '@/types/users';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { PricingPlanInfo } from '@/types/common';

import UpgradeInfoStep from '@poc/components/dialogs/upgradeAccountFlow/UpgradeInfoStep.vue';
import UpgradeOptionsStep from '@poc/components/dialogs/upgradeAccountFlow/UpgradeOptionsStep.vue';
import AddCreditCardStep from '@poc/components/dialogs/upgradeAccountFlow/AddCreditCardStep.vue';
import AddTokensStep from '@poc/components/dialogs/upgradeAccountFlow/AddTokensStep.vue';
import SuccessStep from '@poc/components/dialogs/upgradeAccountFlow/SuccessStep.vue';
import PricingPlanSelectionStep from '@poc/components/dialogs/upgradeAccountFlow/PricingPlanSelectionStep.vue';
import PricingPlanStep from '@poc/components/dialogs/upgradeAccountFlow/PricingPlanStep.vue';

enum UpgradeAccountStep {
    Info = 'infoStep',
    Options = 'optionsStep',
    AddCC = 'addCCStep',
    AddTokens = 'addTokensStep',
    Success = 'successStep',
    PricingPlanSelection = 'pricingPlanSelectionStep',
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
const plan = ref<PricingPlanInfo>();
const content = ref<HTMLElement | null>(null);

const props = withDefaults(defineProps<{
    modelValue: boolean,
    scrim: boolean,
}>(), {
    scrim: true,
});

const emit = defineEmits<{
    'update:modelValue': [value: boolean];
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const stepTitles = computed(() => {
    return {
        [UpgradeAccountStep.Info]: 'Upgrade',
        [UpgradeAccountStep.Options]: 'Upgrade to Pro',
        [UpgradeAccountStep.AddCC]: 'Add Credit Card',
        [UpgradeAccountStep.AddTokens]: 'Add Storj Tokens',
        [UpgradeAccountStep.Success]: 'Success',
        [UpgradeAccountStep.PricingPlanSelection]: 'Upgrade',
        [UpgradeAccountStep.PricingPlan]: plan.value?.title || '',
    };
});

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

function onSelectPricingPlan(p: PricingPlanInfo) {
    plan.value = p;
    setStep(UpgradeAccountStep.PricingPlan);
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
        notify.notifyError(error);
        setStep(UpgradeAccountStep.Options);
        loading.value = false;
        return;
    }
    if (!pkgAvailable) {
        setStep(UpgradeAccountStep.Options);
        loading.value = false;
        return;
    }

    setStep(UpgradeAccountStep.PricingPlanSelection);

    loading.value = false;
}

watch(content, (value) => {
    if (!value) {
        setStep(UpgradeAccountStep.Info);
    }
});
</script>
