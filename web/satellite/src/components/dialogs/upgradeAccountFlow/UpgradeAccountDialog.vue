// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
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
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item>
                <v-window v-model="step" :touch="false" class="no-overflow">
                    <v-window-item :value="UpgradeAccountStep.Info">
                        <UpgradeInfoStep
                            :loading="isLoading"
                            :is-member-upgrade="isMemberUpgrade"
                            @upgrade="upgrade"
                            @start-free-trial="onStartFreeTrial"
                        />
                    </v-window-item>

                    <v-window-item :value="UpgradeAccountStep.Options">
                        <v-tabs
                            v-model="paymentTab"
                            color="primary"
                            show-arrows
                            class="border-b-thin mb-3"
                        >
                            <v-tab>
                                Credit Card
                            </v-tab>
                            <v-tab>
                                STORJ Tokens
                            </v-tab>
                        </v-tabs>
                        <v-window v-model="paymentTab" :touch="false">
                            <v-window-item :value="PaymentOption.CreditCard">
                                <PricingPlanStep
                                    v-model:loading="isLoading"
                                    :plan="plan"
                                    @back="setStep(UpgradeAccountStep.Info)"
                                    @success="() => setStep(UpgradeAccountStep.Success)"
                                />
                            </v-window-item>
                            <v-window-item :value="PaymentOption.StorjTokens">
                                <v-card :loading="isLoading" class="pa-1" variant="flat" :class="{'no-border pa-0': !isLoading}">
                                    <AddTokensStep
                                        v-if="!isLoading"
                                        @back="() => setStep(UpgradeAccountStep.Info)"
                                        @success="onAddTokensSuccess"
                                    />
                                </v-card>
                            </v-window-item>
                        </v-window>
                    </v-window-item>

                    <v-window-item :value="UpgradeAccountStep.Success">
                        <SuccessStep @continue="model = false" />
                    </v-window-item>

                    <v-window-item :value="UpgradeAccountStep.PricingPlan">
                        <PricingPlanStep
                            v-model:loading="isLoading"
                            :plan="plan"
                            @close="model = false"
                            @back="setStep(UpgradeAccountStep.Info)"
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
    VTab,
    VTabs,
    VWindow,
    VWindowItem,
} from 'vuetify/components';
import { useDisplay } from 'vuetify';
import { X } from 'lucide-vue-next';

import { useBillingStore } from '@/store/modules/billingStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { PricingPlanInfo, PricingPlanType } from '@/types/common';
import { Wallet } from '@/types/payments';
import { useUsersStore } from '@/store/modules/usersStore';
import { useLoading } from '@/composables/useLoading';

import UpgradeInfoStep from '@/components/dialogs/upgradeAccountFlow/UpgradeInfoStep.vue';
import AddTokensStep from '@/components/dialogs/upgradeAccountFlow/AddTokensStep.vue';
import SuccessStep from '@/components/dialogs/upgradeAccountFlow/SuccessStep.vue';
import PricingPlanStep from '@/components/dialogs/upgradeAccountFlow/PricingPlanStep.vue';

enum UpgradeAccountStep {
    Info = 'infoStep',
    Options = 'optionsStep',
    AddCC = 'addCCStep',
    AddTokens = 'addTokensStep',
    Success = 'successStep',
    PricingPlan = 'pricingPlanStep',
}

const analyticsStore = useAnalyticsStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();

const { smAndDown, md } = useDisplay();
const notify = useNotify();

const step = ref<UpgradeAccountStep>(UpgradeAccountStep.Info);
const plan = ref<PricingPlanInfo>();
const content = ref<HTMLElement | null>(null);
const wallet = computed<Wallet>(() => billingStore.state.wallet as Wallet);

enum PaymentOption {
    CreditCard,
    StorjTokens,
}

withDefaults(defineProps<{
    scrim?: boolean,
    isMemberUpgrade?: boolean,
}>(), {
    scrim: true,
    isMemberUpgrade: false,
});

const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const paymentTab = ref<PaymentOption>(PaymentOption.CreditCard);

const stepTitles = computed(() => {
    return {
        [UpgradeAccountStep.Info]: 'Upgrade',
        [UpgradeAccountStep.Options]: 'Add Payment Method',
        [UpgradeAccountStep.AddCC]: 'Add Credit Card',
        [UpgradeAccountStep.AddTokens]: 'Add Storj Tokens',
        [UpgradeAccountStep.Success]: 'Success',
        [UpgradeAccountStep.PricingPlan]: plan.value?.planTitle || '',
    };
});

const maxWidth = computed(() => {
    switch (step.value) {
    case UpgradeAccountStep.Info:
        if (billingStore.state.pricingPlansAvailable) {
            return smAndDown.value ? '' : md.value ? '90%' : '65%';
        }
        return smAndDown.value ? '' : md.value ? '80%' : '55%';
    case UpgradeAccountStep.PricingPlan:
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
const isPaidTier = computed((): boolean => usersStore.state.user.isPaid);

/**
 * Handles starting free trial for Member accounts.
 */
function onStartFreeTrial(): void {
    withLoading(async () => {
        try {
            await billingStore.startFreeTrial();
            await usersStore.getUser();

            notify.success('Your free trial has started!');
            model.value = false;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
        }
    });
}

/**
 * Claims wallet and sets add token step.
 */
function onAddTokens(): void {
    withLoading(async () => {
        try {
            await billingStore.claimWallet();

            analyticsStore.eventTriggered(AnalyticsEvent.ADD_FUNDS_CLICKED);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
        }
    });
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

function upgrade(p: PricingPlanInfo) {
    if (isLoading.value) return;

    plan.value = p;

    setStep(p.type === PricingPlanType.PARTNER ? UpgradeAccountStep.PricingPlan : UpgradeAccountStep.Options);
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
</script>

<style scoped lang="scss">
.no-border {
    border: 0 !important;
}

.v-overlay .v-card .no-overflow {
    overflow-y: hidden !important;
}
</style>
