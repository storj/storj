// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog :model-value="shouldShowSetupDialog" fullscreen persistent transition="fade-transition" scrollable>
        <v-card>
            <v-card-item class="pa-1" :class="{ 'h-100': step === OnboardingStep.SetupComplete }">
                <v-window v-model="step" :touch="false">
                    <v-window-item :value="OnboardingStep.AccountInfo">
                        <account-info-step
                            :ref="stepInfos[OnboardingStep.AccountInfo].ref"
                            v-model:name="name"
                            v-model:company-name="companyName"
                            v-model:storage-needs="storageNeeds"
                            v-model:have-sales-contact="haveSalesContact"
                            :loading="isLoading"
                            @next="toNextStep"
                        />
                    </v-window-item>

                    <template v-if="configStore.billingEnabled">
                        <v-window-item :value="OnboardingStep.PlanTypeSelection">
                            <account-type-step
                                @free-click="() => onSelectPricingPlan(FREE_PLAN_INFO)"
                                @pro-click="() => onSelectPricingPlan(proPlanInfo)"
                                @pkg-click="() => onSelectPricingPlan(pricingPlan)"
                                @back="toPrevStep"
                            />
                        </v-window-item>

                        <v-window-item :value="OnboardingStep.PaymentMethodSelection">
                            <v-container>
                                <v-row justify="center">
                                    <v-col class="text-center py-4">
                                        <icon-storj-logo height="50" width="50" class="rounded-xlg bg-background pa-2 border" />
                                        <div class="text-overline mt-2 mb-1">
                                            Account Setup
                                        </div>
                                        <h2>Activate your account</h2>
                                    </v-col>
                                </v-row>
                                <v-row class="ma-0" justify="center" align="center">
                                    <v-col cols="12" sm="10" md="8" lg="6" class="pb-0">
                                        <v-tabs
                                            v-if="isProPlan"
                                            v-model="paymentTab"
                                            color="default"
                                            center-active
                                            show-arrows
                                            class="border-b-thin"
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
                                <v-window v-model="paymentTab" :touch="false">
                                    <v-window-item :value="PaymentOption.CreditCard">
                                        <v-row class="ma-0" justify="center" align="center">
                                            <v-col cols="12" sm="10" md="8" lg="6">
                                                <PricingPlanStep
                                                    v-model:loading="isLoading"
                                                    :plan="plan"
                                                    is-account-setup
                                                    @back="toPrevStep"
                                                    @success="toNextStep"
                                                />
                                            </v-col>
                                        </v-row>
                                    </v-window-item>
                                    <v-window-item :value="PaymentOption.StorjTokens">
                                        <v-row justify="center" align="center" class="ma-0 mt-2">
                                            <v-col cols="12" sm="10" md="8" lg="6">
                                                <v-card :loading="isLoading" class="pa-1" variant="flat" :class="{'no-border pa-0': !isLoading}">
                                                    <AddTokensStep
                                                        v-if="!isLoading"
                                                        @back="onBackFromTokens"
                                                        @success="toNextStep"
                                                    />
                                                </v-card>
                                            </v-col>
                                        </v-row>
                                    </v-window-item>
                                </v-window>
                            </v-container>
                        </v-window-item>
                    </template>

                    <v-window-item v-if="satelliteManagedEncryptionEnabled" :value="OnboardingStep.ManagedPassphraseOptIn">
                        <managed-passphrase-opt-in-step
                            :ref="stepInfos[OnboardingStep.ManagedPassphraseOptIn].ref"
                            v-model:manage-mode="passphraseManageMode"
                            :loading="isLoading"
                            @next="toNextStep"
                        />
                    </v-window-item>

                    <v-window-item :value="OnboardingStep.SetupComplete">
                        <success-step
                            :ref="stepInfos[OnboardingStep.SetupComplete].ref"
                            :loading="isLoading"
                            @finish="isAccountSetup = false"
                        />
                    </v-window-item>
                </v-window>
            </v-card-item>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import {
    computed,
    onBeforeMount,
    ref,
    watch,
} from 'vue';
import {
    VCard,
    VCardItem,
    VCol,
    VContainer,
    VDialog,
    VRow,
    VTab,
    VTabs,
    VWindow,
    VWindowItem,
} from 'vuetify/components';

import { useUsersStore } from '@/store/modules/usersStore';
import {
    AccountSetupStorageNeeds,
    ACCOUNT_SETUP_STEPS,
    OnboardingStep,
    SetUserSettingsData,
    UserSettings,
} from '@/types/users';
import { FREE_PLAN_INFO, PricingPlanInfo, PricingPlanType, StepInfo } from '@/types/common';
import { useConfigStore } from '@/store/modules/configStore';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { ManagePassphraseMode } from '@/types/projects';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { Wallet } from '@/types/payments';

import SuccessStep from '@/components/dialogs/accountSetupSteps/SuccessStep.vue';
import PricingPlanStep from '@/components/dialogs/upgradeAccountFlow/PricingPlanStep.vue';
import ManagedPassphraseOptInStep from '@/components/dialogs/accountSetupSteps/ManagedPassphraseOptInStep.vue';
import AccountTypeStep from '@/components/dialogs/accountSetupSteps/AccountTypeStep.vue';
import IconStorjLogo from '@/components/icons/IconStorjLogo.vue';
import AddTokensStep from '@/components/dialogs/upgradeAccountFlow/AddTokensStep.vue';
import AccountInfoStep from '@/components/dialogs/accountSetupSteps/AccountInfoStep.vue';

enum PaymentOption {
    CreditCard,
    StorjTokens,
}

const billingStore = useBillingStore();
const configStore = useConfigStore();
const projectsStore = useProjectsStore();
const userStore = useUsersStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const step = ref<OnboardingStep>(OnboardingStep.AccountInfo);
const plan = ref<PricingPlanInfo>();
const passphraseManageMode = ref<ManagePassphraseMode>('auto');
const paymentTab = ref<PaymentOption>(PaymentOption.CreditCard);
const isAccountSetup = ref<boolean>(false);
const name = ref<string>('');
const companyName = ref<string>('');
const storageNeeds = ref<AccountSetupStorageNeeds | undefined>(undefined);
const haveSalesContact = ref<boolean>(false);

const pricingPlan = computed<PricingPlanInfo | null>(() => billingStore.state.pricingPlanInfo);
const pkgAvailable = computed<boolean>(() => billingStore.state.pricingPlansAvailable);
const proPlanInfo = computed<PricingPlanInfo>(() => billingStore.proPlanInfo);
const isProPlan = computed<boolean>(() => plan.value?.type === PricingPlanType.PRO);
const isFreePlan = computed<boolean>(() => plan.value?.type === PricingPlanType.FREE);
const wallet = computed<Wallet>(() => billingStore.state.wallet as Wallet);
const shouldShowSetupDialog = computed<boolean>(() => {
    // settings are fetched on the projects page.
    const onboardingEnd = userStore.state.settings.onboardingEnd;
    const currentStep = userSettings.value.onboardingStep;

    if (onboardingEnd || (currentStep && !ACCOUNT_SETUP_STEPS.some(s => s === currentStep))) {
        return false;
    }

    return isAccountSetup.value;
});
const userSettings = computed<UserSettings>(() => userStore.state.settings as UserSettings);
const satelliteManagedEncryptionEnabled = computed<boolean>(() => configStore.state.config.satelliteManagedEncryptionEnabled);
const allowManagedPassphraseStep = computed<boolean>(() => satelliteManagedEncryptionEnabled.value && projectsStore.state.projects.length === 0);
const defaultNextStep = computed<OnboardingStep>(() => {
    return allowManagedPassphraseStep.value ? OnboardingStep.ManagedPassphraseOptIn : OnboardingStep.SetupComplete;
});
const accountInfoNextStep = computed<OnboardingStep>(() => {
    // If billing isn’t on, we always take the default step.
    if (!configStore.billingEnabled) return defaultNextStep.value;
    return OnboardingStep.PlanTypeSelection;
});
const isMemberAccount = computed<boolean>(() => userStore.state.user.isMember);

const stepInfos: Record<string, StepInfo<OnboardingStep>> = {
    [OnboardingStep.AccountInfo]: new StepInfo<OnboardingStep>({
        next: () => {
            if (isMemberAccount.value) {
                return OnboardingStep.SetupComplete; // Skip to the end for member accounts.
            } else {
                return accountInfoNextStep.value;
            }
        },
        beforeNext: async () => {
            await stepInfos[OnboardingStep.AccountInfo].ref.value?.setup?.();

            const update: SetUserSettingsData = { onboardingStep: accountInfoNextStep.value };
            if (!userSettings.value.onboardingStart) {
                update.onboardingStart = true;
            }

            if (isMemberAccount.value) {
                update.onboardingStep = OnboardingStep.SetupComplete; // Skip to the end for member accounts.
            }

            await userStore.updateSettings(update);
        },
    }),
    [OnboardingStep.PlanTypeSelection]: new StepInfo<OnboardingStep>({
        prev: () => OnboardingStep.AccountInfo,
        next: () => {
            if (!isFreePlan.value) return OnboardingStep.PaymentMethodSelection;
            return defaultNextStep.value;
        },
        beforeNext: async () => {
            if (isFreePlan.value) {
                await userStore.updateSettings({ onboardingStep: defaultNextStep.value });
            }
        },
        noRef: true,
    }),
    [OnboardingStep.PaymentMethodSelection]: new StepInfo<OnboardingStep>({
        prev: () => OnboardingStep.PlanTypeSelection,
        next: () => defaultNextStep.value,
        beforeNext: async () => {
            await userStore.updateSettings({ onboardingStep: defaultNextStep.value });
        },
        noRef: true,
    }),
    [OnboardingStep.ManagedPassphraseOptIn]: new StepInfo<OnboardingStep>({
        next: () => OnboardingStep.SetupComplete,
        beforeNext: async () => {
            await stepInfos[OnboardingStep.ManagedPassphraseOptIn].ref.value?.setup?.();
            await userStore.updateSettings({ onboardingStep: OnboardingStep.SetupComplete });
        },
    }),
    [OnboardingStep.SetupComplete]: new StepInfo<OnboardingStep>({
        beforeNext: async () => {
            if (isMemberAccount.value) {
                await userStore.updateSettings({ onboardingEnd: true });
                return;
            }

            await stepInfos[OnboardingStep.SetupComplete].ref.value?.setup?.();
        },
    }),
};

function onSelectPricingPlan(p: PricingPlanInfo | null): void {
    if (!p) return;
    plan.value = p;
    toNextStep();
}

function onBackFromTokens(): void {
    toPrevStep();
    paymentTab.value = PaymentOption.CreditCard;
}

/**
 * Claims wallet and sets add token step.
 */
function onAddTokens(): void {
    withLoading(async () => {
        try {
            await billingStore.claimWallet();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETUP_DIALOG);
        }
    });
}

/**
 * Decides whether to move to the success step or the pricing plan selection.
 */
function toNextStep(): void {
    const info = stepInfos[step.value];
    if (info.ref.value?.validate?.() === false) {
        return;
    }

    withLoading(async () => {
        try {
            await info.beforeNext?.();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETUP_DIALOG);
            return;
        }

        if (info.next?.value) {
            step.value = info.next.value;
        }
    });
}

function toPrevStep(): void {
    const info = stepInfos[step.value];
    if (info.prev?.value) {
        step.value = info.prev.value;
    }
    plan.value = undefined;
}

/**
 * Figure out the initial setup step.
 */
onBeforeMount(() => {
    if (!satelliteManagedEncryptionEnabled.value) {
        passphraseManageMode.value = 'manual';
    }

    const currentStep = userSettings.value.onboardingStep;

    if (userSettings.value.onboardingEnd || (currentStep && !ACCOUNT_SETUP_STEPS.some(s => s === currentStep))) {
        return;
    }

    name.value = userStore.userName ?? '';

    switch (true) {
    case currentStep === OnboardingStep.SetupComplete ||
        (currentStep === OnboardingStep.ManagedPassphraseOptIn && !allowManagedPassphraseStep.value):
        step.value = OnboardingStep.SetupComplete;
        break;
    case ACCOUNT_SETUP_STEPS.some(s => s === currentStep):
        step.value = currentStep as OnboardingStep;
        break;
    case !userStore.userName:
        step.value = OnboardingStep.AccountInfo;
        break;
    case pkgAvailable.value:
        step.value = OnboardingStep.PlanTypeSelection;
        break;
    }

    isAccountSetup.value = true;
});

watch(paymentTab, newTab => {
    if (newTab === PaymentOption.StorjTokens && !wallet.value.address) onAddTokens();
});
</script>

<style scoped lang="scss">
.no-border {
    border: 0 !important;
}
</style>
