// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog :model-value="shouldShowSetupDialog" :height="dialogHeight" :width="dialogWidth" persistent transition="fade-transition" scrollable>
        <v-card
            ref="innerContent"
            :title="step === OnboardingStep.PricingPlanSelection ? 'Select a pricing plan' : ''"
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
                    <v-window-item :value="OnboardingStep.AccountTypeSelection">
                        <choice-step @next="toNextStep" />
                    </v-window-item>

                    <!-- Business step -->
                    <v-window-item :value="OnboardingStep.BusinessAccountForm">
                        <business-step @back="toNextStep(OnboardingStep.AccountTypeSelection)" @next="toNextStep" />
                    </v-window-item>

                    <!-- Personal step -->
                    <v-window-item :value="OnboardingStep.PersonalAccountForm">
                        <personal-step @back="toNextStep(OnboardingStep.AccountTypeSelection)" @next="toNextStep" />
                    </v-window-item>

                    <v-window-item v-if="satelliteManagedEncryptionEnabled" :value="OnboardingStep.ManagedPassphraseOptIn">
                        <managed-passphrase-opt-in-step
                            v-model:manage-mode="passphraseManageMode"
                            @back="toNextStep(OnboardingStep.AccountTypeSelection)" @next="toNextStep"
                        />
                    </v-window-item>

                    <!-- Pricing plan steps -->
                    <v-window-item :value="OnboardingStep.PricingPlanSelection">
                        <pricing-plan-selection-step
                            show-free-plan
                            @select="onSelectPricingPlan"
                        />
                    </v-window-item>

                    <v-window-item :value="OnboardingStep.PricingPlan">
                        <PricingPlanStep
                            :plan="plan"
                            @success="toNextStep"
                            @back="() => toNextStep(OnboardingStep.PricingPlanSelection)"
                        />
                    </v-window-item>

                    <!-- Final step -->
                    <v-window-item :value="OnboardingStep.SetupComplete">
                        <success-step />
                    </v-window-item>
                </v-window>
            </v-card-item>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, computed, onBeforeMount, ref, watch } from 'vue';
import { VCard, VCardItem, VContainer, VDialog, VProgressCircular, VRow, VWindow, VWindowItem } from 'vuetify/components';

import { useUsersStore } from '@/store/modules/usersStore';
import { ACCOUNT_SETUP_STEPS, ONBOARDING_STEPPER_STEPS, OnboardingStep, UserSettings } from '@/types/users';
import { PricingPlanInfo } from '@/types/common';
import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@/store/modules/appStore';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { ManagePassphraseMode } from '@/types/projects';

import ChoiceStep from '@/components/dialogs/accountSetupSteps/ChoiceStep.vue';
import BusinessStep from '@/components/dialogs/accountSetupSteps/BusinessStep.vue';
import PersonalStep from '@/components/dialogs/accountSetupSteps/PersonalStep.vue';
import SuccessStep from '@/components/dialogs/accountSetupSteps/SuccessStep.vue';
import PricingPlanSelectionStep from '@/components/dialogs/upgradeAccountFlow/PricingPlanSelectionStep.vue';
import PricingPlanStep from '@/components/dialogs/upgradeAccountFlow/PricingPlanStep.vue';
import ManagedPassphraseOptInStep from '@/components/dialogs/accountSetupSteps/ManagedPassphraseOptInStep.vue';

const appStore = useAppStore();
const billingStore = useBillingStore();
const configStore = useConfigStore();
const projectsStore = useProjectsStore();
const userStore = useUsersStore();

const { isLoading, withLoading } = useLoading();

const innerContent = ref<Component | null>(null);
const step = ref<OnboardingStep>(OnboardingStep.AccountTypeSelection);
const plan = ref<PricingPlanInfo>();
const passphraseManageMode = ref<ManagePassphraseMode>('auto');

const pkgAvailable = computed(() => billingStore.state.pricingPlansAvailable);

/**
 * Indicates if satellite managed encryption passphrase is enabled.
 */
const satelliteManagedEncryptionEnabled = computed<boolean>(() => configStore.state.config.satelliteManagedEncryptionEnabled);

/**
 * Indicates whether to allow progression to managed passphrase opt in step.
 */
const allowManagedPassphraseStep = computed(() => satelliteManagedEncryptionEnabled.value && projectsStore.state.projects.length === 0);

const shouldShowSetupDialog = computed(() => {
    // settings are fetched on the projects page.
    const onboardingEnd = userStore.state.settings.onboardingEnd;
    if (onboardingEnd || !!ONBOARDING_STEPPER_STEPS.find(s => s === userSettings.value.onboardingStep)) {
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

    case OnboardingStep.AccountTypeSelection:
    case OnboardingStep.BusinessAccountForm:
    case OnboardingStep.PersonalAccountForm:
    case OnboardingStep.ManagedPassphraseOptIn:
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

    case OnboardingStep.AccountTypeSelection:
    case OnboardingStep.BusinessAccountForm:
    case OnboardingStep.PersonalAccountForm:
        return '87%';

    case OnboardingStep.PricingPlanSelection:
        return '720px';
    case OnboardingStep.PricingPlan:
        return '460px';
    case OnboardingStep.SetupComplete:
        return '540px';
    default:
        return '';
    }
});

async function onSelectPricingPlan(p: PricingPlanInfo) {
    plan.value = p;
    toNextStep(OnboardingStep.PricingPlan);
}

/**
 * Decides whether to move to the success step or the pricing plan selection.
 */
function toNextStep(next?: OnboardingStep) {
    if (!userSettings.value.onboardingStart) {
        userStore.updateSettings({ onboardingStart: true });
    }

    const isForm = step.value === OnboardingStep.PersonalAccountForm || step.value === OnboardingStep.BusinessAccountForm;
    if (!next) {
        if (isForm && allowManagedPassphraseStep.value) {
            step.value = OnboardingStep.ManagedPassphraseOptIn;
        } else if (isForm && pkgAvailable.value) {
            step.value = OnboardingStep.PricingPlanSelection;
        } else if (pkgAvailable.value && step.value === OnboardingStep.ManagedPassphraseOptIn) {
            step.value = OnboardingStep.PricingPlanSelection;
        } else {
            step.value = OnboardingStep.SetupComplete;
        }
    } else {
        step.value = next;
    }

    if (step.value === OnboardingStep.PricingPlan) {
        return;
    }
    userStore.updateSettings({ onboardingStep: step.value });
}

/**
 * Figure out whether this dialog should show and the initial setup step.
 */
onBeforeMount(() => {
    withLoading(async () => {
        if (userSettings.value.onboardingEnd || !!ONBOARDING_STEPPER_STEPS.find(s => s === userSettings.value.onboardingStep)) {
            return;
        }

        if (userSettings.value.onboardingStep === OnboardingStep.SetupComplete) {
            step.value = OnboardingStep.SetupComplete;
            appStore.toggleAccountSetup(true);
            return;
        }

        if (userSettings.value.onboardingStep === OnboardingStep.ManagedPassphraseOptIn && !allowManagedPassphraseStep.value) {
            step.value = OnboardingStep.SetupComplete;
        } else if (userSettings.value.onboardingStep === OnboardingStep.PricingPlanSelection && !pkgAvailable.value) {
            step.value = OnboardingStep.SetupComplete;
        } else if (ACCOUNT_SETUP_STEPS.find(s => s === userSettings.value.onboardingStep)) {
            step.value = userSettings.value.onboardingStep as OnboardingStep;
        } else if (!userStore.userName) {
            step.value = OnboardingStep.AccountTypeSelection;
        } else if (pkgAvailable.value) {
            step.value = OnboardingStep.PricingPlanSelection;
        }

        appStore.toggleAccountSetup(true);
    });
});

watch(innerContent, comp => {
    if (comp) {
        if (!satelliteManagedEncryptionEnabled.value) {
            passphraseManageMode.value = 'manual';
        }
        return;
    }
    step.value = OnboardingStep.AccountTypeSelection;
});
</script>
