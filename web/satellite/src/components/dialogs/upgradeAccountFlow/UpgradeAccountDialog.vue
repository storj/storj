// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        scrollable
        min-width="320px"
        :max-width="maxWidth"
        transition="fade-transition"
        persistent
        :scrim="scrim"
    >
        <v-card ref="content">
            <v-card-item class="pa-6">
                <template v-if="step === UpgradeAccountStep.Success" #prepend>
                    <img class="d-block" src="@/assets/icon-success.svg" alt="success">
                </template>
                <v-card-title class="font-weight-bold">{{ stepTitles[step] }}</v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="loading"
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
                        <v-container>
                            <v-row justify="center" align="center">
                                <v-col cols="12" sm="12" md="10" lg="8">
                                    <v-tabs
                                        v-model="paymentTab"
                                        color="default"
                                        center-active
                                        show-arrows
                                        grow
                                    >
                                        <v-tab>
                                            Credit Card
                                        </v-tab>
                                        <v-tab>
                                            STORJ tokens
                                        </v-tab>
                                    </v-tabs>
                                </v-col>
                            </v-row>
                            <v-window v-model="paymentTab">
                                <v-window-item :value="PaymentOption.CreditCard">
                                    <v-row justify="center" align="center">
                                        <v-col cols="12" sm="12" md="10" lg="8">
                                            <AddCreditCardStep
                                                v-model:loading="loading"
                                                @success="() => setStep(UpgradeAccountStep.Success)"
                                                @back="() => setStep(UpgradeAccountStep.Info)"
                                            />
                                        </v-col>
                                    </v-row>
                                </v-window-item>
                                <v-window-item :value="PaymentOption.StorjTokens">
                                    <v-row justify="center" align="center" class="mt-2">
                                        <v-col cols="12" sm="12" md="10" lg="8">
                                            <v-card :loading="loading" class="pa-1" :class="{'no-border pa-0': !loading}">
                                                <AddTokensStep
                                                    v-if="!loading"
                                                    @back="() => setStep(UpgradeAccountStep.Info)"
                                                    @success="onAddTokensSuccess"
                                                />
                                            </v-card>
                                        </v-col>
                                    </v-row>
                                </v-window-item>
                            </v-window>
                        </v-container>
                    </v-window-item>

                    <v-window-item :value="UpgradeAccountStep.Success">
                        <SuccessStep @continue="model = false" />
                    </v-window-item>

                    <v-window-item :value="UpgradeAccountStep.PricingPlanSelection">
                        <PricingPlanSelectionStep @select="onSelectPricingPlan" />
                    </v-window-item>

                    <v-window-item :value="UpgradeAccountStep.PricingPlan">
                        <PricingPlanStep
                            v-model:loading="loading"
                            :plan="plan"
                            @close="model = false"
                            @back="setStep(UpgradeAccountStep.PricingPlanSelection)"
                        />
                    </v-window-item>
                </v-window>
            </v-card-item>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardItem,
    VCardTitle,
    VDialog,
    VDivider,
    VWindow,
    VWindowItem,
    VRow,
    VCol,
    VTabs,
    VTab,
    VContainer,
} from 'vuetify/components';

import { useBillingStore } from '@/store/modules/billingStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { PricingPlanInfo } from '@/types/common';
import { Wallet } from '@/types/payments';
import { useUsersStore } from '@/store/modules/usersStore';

import UpgradeInfoStep from '@/components/dialogs/upgradeAccountFlow/UpgradeInfoStep.vue';
import AddCreditCardStep from '@/components/dialogs/upgradeAccountFlow/AddCreditCardStep.vue';
import AddTokensStep from '@/components/dialogs/upgradeAccountFlow/AddTokensStep.vue';
import SuccessStep from '@/components/dialogs/upgradeAccountFlow/SuccessStep.vue';
import PricingPlanSelectionStep from '@/components/dialogs/upgradeAccountFlow/PricingPlanSelectionStep.vue';
import PricingPlanStep from '@/components/dialogs/upgradeAccountFlow/PricingPlanStep.vue';

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
const billingStore = useBillingStore();
const usersStore = useUsersStore();

const notify = useNotify();

const step = ref<UpgradeAccountStep>(UpgradeAccountStep.Info);
const loading = ref<boolean>(false);
const plan = ref<PricingPlanInfo>();
const content = ref<HTMLElement | null>(null);
const wallet = computed<Wallet>(() => billingStore.state.wallet as Wallet);

enum PaymentOption {
    CreditCard,
    StorjTokens,
}

const paymentTab = ref<PaymentOption>(PaymentOption.CreditCard);

withDefaults(defineProps<{
    scrim?: boolean,
}>(), {
    scrim: true,
});

const model = defineModel<boolean>({ required: true });

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

const maxWidth = computed(() => {
    switch (step.value) {
    case UpgradeAccountStep.Info:
    case UpgradeAccountStep.PricingPlanSelection:
    case UpgradeAccountStep.AddTokens:
    case UpgradeAccountStep.Options:
        return '720px';
    default:
        return '460px';
    }
});

/**
 * Returns whether the user is in paid tier.
 */
const isPaidTier = computed((): boolean => usersStore.state.user.paidTier);

/**
 * Claims wallet and sets add token step.
 */
async function onAddTokens(): Promise<void> {
    if (loading.value) return;

    loading.value = true;

    try {
        await billingStore.claimWallet();

        analyticsStore.eventTriggered(AnalyticsEvent.ADD_FUNDS_CLICKED);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
    }

    loading.value = false;
}

function onAddTokensSuccess(): void {
    if (isPaidTier.value) {
        setStep(UpgradeAccountStep.Success);
        return;
    }

    model.value = false;
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
    const newStep = billingStore.state.pricingPlansAvailable ? UpgradeAccountStep.PricingPlanSelection : UpgradeAccountStep.Options;
    setStep(newStep);
}

watch(paymentTab, newTab => {
    if (newTab === PaymentOption.StorjTokens && !wallet.value.address) onAddTokens();
});

watch(content, (value) => {
    if (!value) {
        setStep(UpgradeAccountStep.Info);
        return;
    }
});

defineExpose({ setSecondStep });
</script>

<style scoped lang="scss">
.no-border {
    border: 0 !important;
}
</style>
