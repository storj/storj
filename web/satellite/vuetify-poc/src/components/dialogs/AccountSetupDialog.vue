// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog :model-value="shouldShowSetupDialog" :height="dialogHeight" :width="dialogWidth" persistent transition="fade-transition" scrollable>
        <v-card
            ref="innerContent"
            :title="step === AccountSetupStep.PricingPlanSelection ? 'Select a pricing plan' : ''"
        >
            <v-card-item class="py-4">
                <v-container
                    v-if="isLoading"
                    class="fill-height"
                    fluid
                >
                    <v-row justify="center" align="center">
                        <v-progress-circular indeterminate />
                    </v-row>
                </v-container>

                <v-window v-else v-model="step">
                    <!-- Choice step -->
                    <v-window-item :value="step === AccountSetupStep.Choice">
                        <choice-step @next="toNextStep" />
                    </v-window-item>

                    <!-- Business step -->
                    <v-window-item :value="AccountSetupStep.Business">
                        <business-step @next="toNextStep" />
                    </v-window-item>

                    <!-- Personal step -->
                    <v-window-item :value="AccountSetupStep.Personal">
                        <personal-step @next="toNextStep" />
                    </v-window-item>

                    <!-- Pricing plan steps -->
                    <v-window-item :value="AccountSetupStep.PricingPlanSelection">
                        <pricing-plan-selection-step
                            show-free-plan
                            @select="onSelectPricingPlan"
                        />
                    </v-window-item>

                    <v-window-item :value="AccountSetupStep.PricingPlan">
                        <PricingPlanStep
                            :plan="plan"
                            @success="() => toNextStep(AccountSetupStep.Success)"
                            @back="() => toNextStep(AccountSetupStep.PricingPlanSelection)"
                        />
                    </v-window-item>

                    <!-- Final step -->
                    <v-window-item :value="AccountSetupStep.Success">
                        <success-step @continue="finishSetup" />
                    </v-window-item>
                </v-window>
            </v-card-item>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, computed, onBeforeMount, ref, watch } from 'vue';
import { VCard, VCardItem, VContainer, VDialog, VProgressCircular, VRow, VWindow, VWindowItem } from 'vuetify/components';

import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { AccountSetupStep, UserSettings } from '@/types/users';
import { PricingPlanInfo } from '@/types/common';
import { useConfigStore } from '@/store/modules/configStore';
import { PaymentsHttpApi } from '@/api/payments';
import { useAppStore } from '@poc/store/appStore';
import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBillingStore } from '@/store/modules/billingStore';

import ChoiceStep from '@poc/components/dialogs/accountSetupSteps/ChoiceStep.vue';
import BusinessStep from '@poc/components/dialogs/accountSetupSteps/BusinessStep.vue';
import PersonalStep from '@poc/components/dialogs/accountSetupSteps/PersonalStep.vue';
import SuccessStep from '@poc/components/dialogs/accountSetupSteps/SuccessStep.vue';
import PricingPlanSelectionStep from '@poc/components/dialogs/upgradeAccountFlow/PricingPlanSelectionStep.vue';
import PricingPlanStep from '@poc/components/dialogs/upgradeAccountFlow/PricingPlanStep.vue';

const payments: PaymentsHttpApi = new PaymentsHttpApi();

const appStore = useAppStore();
const billingStore = useBillingStore();
const configStore = useConfigStore();
const userStore = useUsersStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const innerContent = ref<Component | null>(null);
const step = ref<AccountSetupStep>(AccountSetupStep.Choice);
const plan = ref<PricingPlanInfo>();

const pkgAvailable = ref(false);

const shouldShowSetupDialog = computed(() => {
    // settings are fetched on the projects page.
    const onboardingEnd = userStore.state.settings.onboardingEnd;
    if (onboardingEnd) {
        return false;
    }

    return appStore.state.isAccountSetupDialogShown;
});

const userSettings = computed(() => userStore.state.settings as UserSettings);

/**
 * step-dynamic dialog height
 */
const dialogHeight = computed(() => {
    switch (step.value) {

    case AccountSetupStep.Choice:
    case AccountSetupStep.Business:
    case AccountSetupStep.Personal:
        return '87%';
    default:
        return 'auto';
    }
});

/**
 * step-dynamic dialog width
 */
const dialogWidth = computed(() => {
    switch (step.value) {

    case AccountSetupStep.PricingPlanSelection:
        return '720px';
    case AccountSetupStep.PricingPlan:
    case AccountSetupStep.Success:
        return '460px';
    default:
        return '';
    }
});

async function onSelectPricingPlan(p: PricingPlanInfo) {
    plan.value = p;
    toNextStep(AccountSetupStep.PricingPlan);
}

/**
 * Decides whether to move to the success step or the pricing plan selection.
 */
function toNextStep(next: AccountSetupStep) {
    if (step.value !== AccountSetupStep.Personal && step.value !== AccountSetupStep.Business) {
        step.value = next;
        return;
    }

    if (next === AccountSetupStep.Choice) {
        step.value = next;
        return;
    }

    if (!userSettings.value.onboardingStart) {
        userStore.updateSettings({ onboardingStart: true });
    }

    if (!pkgAvailable.value) {
        step.value = AccountSetupStep.Success;
        return;
    }

    step.value = AccountSetupStep.PricingPlanSelection;
}

function finishSetup() {
    appStore.toggleAccountSetup(false);
    userStore.updateSettings({ onboardingEnd: true });
}

/**
 * Figure out whether this dialog should show and the initial setup step.
 */
onBeforeMount(() => {
    withLoading(async () => {
        if (!userStore.state.user.email) {
            await userStore.getUser();
        }

        if (configStore.state.config.billingFeaturesEnabled) {
            try {
                await billingStore.setupAccount();
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETUP_DIALOG);
            }

            const pricingPkgsEnabled = configStore.state.config.pricingPackagesEnabled;
            if (pricingPkgsEnabled && userStore.state.user.partner) {
                try {
                    pkgAvailable.value = await payments.pricingPackageAvailable();
                } catch (error) {
                    notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETUP_DIALOG);
                    return;
                }
            }
        }

        if (!userStore.userName) {
            step.value = AccountSetupStep.Choice;
        } else if (pkgAvailable.value) {
            step.value = AccountSetupStep.PricingPlanSelection;
        }

        appStore.toggleAccountSetup(true);
    });
});

watch(innerContent, comp => {
    if (comp) return;
    step.value = AccountSetupStep.Choice;
});
</script>
