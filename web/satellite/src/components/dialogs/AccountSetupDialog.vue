// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog :model-value="shouldShowSetupDialog" height="87%" width="87%" persistent transition="fade-transition" scrollable>
        <v-card
            ref="innerContent"
        >
            <v-card-item class="py-4" :class="{ 'h-100': step === OnboardingStep.SetupComplete }">
                <v-window v-model="step">
                    <!-- Choice step -->
                    <v-window-item :value="OnboardingStep.AccountTypeSelection">
                        <choice-step @select="onChoiceSelect" />
                    </v-window-item>

                    <!-- Business step -->
                    <v-window-item :value="OnboardingStep.BusinessAccountForm">
                        <business-step
                            :ref="stepInfos[OnboardingStep.BusinessAccountForm].ref"
                            v-model:first-name="firstName"
                            v-model:last-name="lastName"
                            v-model:company-name="companyName"
                            v-model:position="position"
                            v-model:employee-count="employeeCount"
                            v-model:storage-needs="storageNeeds"
                            v-model:functional-area="functionalArea"
                            v-model:have-sales-contact="haveSalesContact"
                            v-model:interested-in-partnering="interestedInPartnering"
                            v-model:use-case="useCase"
                            v-model:other-use-case="otherUseCase"
                            :loading="isLoading"
                            @back="toPrevStep"
                            @next="toNextStep"
                        />
                    </v-window-item>

                    <!-- Personal step -->
                    <v-window-item :value="OnboardingStep.PersonalAccountForm">
                        <personal-step
                            :ref="stepInfos[OnboardingStep.PersonalAccountForm].ref"
                            v-model:name="firstName"
                            v-model:use-case="useCase"
                            v-model:other-use-case="otherUseCase"
                            :loading="isLoading"
                            @back="toPrevStep"
                            @next="toNextStep"
                        />
                    </v-window-item>

                    <v-window-item :value="OnboardingStep.PlanTypeSelection">
                        <account-type-step
                            @free-click="() => onSelectPricingPlan(FREE_PLAN_INFO)"
                            @pro-click="toNextStep"
                            @back="toPrevStep"
                        />
                    </v-window-item>

                    <v-window-item :value="OnboardingStep.PaymentMethodSelection">
                        <v-container>
                            <v-row justify="center">
                                <v-col class="text-center py-4">
                                    <icon-storj-logo />
                                    <div class="text-overline mt-2 mb-1">
                                        Account Setup
                                    </div>
                                    <h2>Activate your account</h2>
                                </v-col>
                            </v-row>
                            <v-row justify="center" align="center">
                                <v-col cols="12" sm="8" md="6" lg="4">
                                    <p class="text-body-2 my-2">
                                        Add a credit card to activate your Pro Account, or deposit
                                        more than $10 in STORJ tokens to upgrade and get 10% bonus
                                        on your STORJ tokens deposit.
                                    </p>
                                    <v-row justify="center" class="pb-5 pt-3">
                                        <v-col>
                                            <v-btn
                                                variant="flat"
                                                color="primary"
                                                :disabled="isLoading"
                                                :prepend-icon="CreditCard"
                                                block
                                                @click="() => onSelectPricingPlan(PRO_PLAN_INFO)"
                                            >
                                                Add Credit Card
                                            </v-btn>
                                        </v-col>
                                        <v-col>
                                            <v-btn
                                                variant="flat"
                                                :loading="isLoading"
                                                :prepend-icon="CirclePlus"
                                                block
                                                @click="onAddTokens"
                                            >
                                                Add STORJ Tokens
                                            </v-btn>
                                        </v-col>
                                    </v-row>
                                    <div class="pb-4">
                                        <v-btn
                                            block
                                            variant="text"
                                            color="default"
                                            :prepend-icon="ChevronLeft"
                                            :disabled="isLoading"
                                            @click="toPrevStep"
                                        >
                                            Back
                                        </v-btn>
                                    </div>
                                </v-col>
                            </v-row>
                        </v-container>
                    </v-window-item>

                    <v-window-item :value="OnboardingStep.AddTokens">
                        <v-container>
                            <v-row justify="center">
                                <v-col class="text-center py-4">
                                    <icon-storj-logo />
                                    <div class="text-overline mt-2 mb-1">
                                        Account Setup
                                    </div>
                                    <h2>Activate your account</h2>
                                </v-col>
                            </v-row>
                            <v-row justify="center" align="center">
                                <v-col cols="12" sm="8" md="6" lg="4">
                                    <AddTokensStep
                                        @back="toPrevStep"
                                        @success="toNextStep"
                                    />
                                </v-col>
                            </v-row>
                        </v-container>
                    </v-window-item>

                    <!-- Pricing plan steps -->
                    <v-window-item :value="OnboardingStep.PricingPlanSelection">
                        <v-container>
                            <v-row justify="center">
                                <v-col class="text-center py-4">
                                    <icon-storj-logo height="50" width="50" class="rounded-xlg bg-background pa-2 border" />
                                    <div class="text-overline mt-2 mb-1">
                                        Pricing Plan
                                    </div>
                                    <h2>Select a pricing plan</h2>
                                </v-col>
                            </v-row>
                            <v-row justify="center" align="center">
                                <pricing-plan-selection-step
                                    show-free-plan
                                    @select="onSelectPricingPlan"
                                />
                            </v-row>
                        </v-container>
                    </v-window-item>

                    <v-window-item :value="OnboardingStep.PricingPlan">
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
                            <v-row justify="center" align="center">
                                <v-col cols="12" sm="8" md="6" lg="4">
                                    <PricingPlanStep
                                        v-model:loading="isLoading"
                                        :plan="plan"
                                        is-account-setup
                                        @back="toPrevStep"
                                        @success="toNextStep"
                                    />
                                </v-col>
                            </v-row>
                        </v-container>
                    </v-window-item>

                    <v-window-item v-if="satelliteManagedEncryptionEnabled" :value="OnboardingStep.ManagedPassphraseOptIn">
                        <managed-passphrase-opt-in-step
                            :ref="stepInfos[OnboardingStep.ManagedPassphraseOptIn].ref"
                            v-model:manage-mode="passphraseManageMode"
                            :loading="isLoading"
                            @next="toNextStep"
                        />
                    </v-window-item>

                    <!-- Final step -->
                    <v-window-item :value="OnboardingStep.SetupComplete">
                        <success-step
                            :ref="stepInfos[OnboardingStep.SetupComplete].ref"
                            :loading="isLoading"
                        />
                    </v-window-item>
                </v-window>
            </v-card-item>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, computed, onBeforeMount, Ref, ref, watch } from 'vue';
import { VBtn, VCard, VCardItem, VCol, VContainer, VDialog, VRow, VWindow, VWindowItem } from 'vuetify/components';
import { ChevronLeft, CirclePlus, CreditCard } from 'lucide-vue-next';

import { useUsersStore } from '@/store/modules/usersStore';
import {
    ACCOUNT_SETUP_STEPS,
    ONBOARDING_STEPPER_STEPS,
    OnboardingStep,
    SetUserSettingsData,
    UserSettings,
} from '@/types/users';
import { FREE_PLAN_INFO, PricingPlanInfo, PricingPlanType, PRO_PLAN_INFO } from '@/types/common';
import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@/store/modules/appStore';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { ManagePassphraseMode } from '@/types/projects';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import ChoiceStep from '@/components/dialogs/accountSetupSteps/ChoiceStep.vue';
import BusinessStep from '@/components/dialogs/accountSetupSteps/BusinessStep.vue';
import PersonalStep from '@/components/dialogs/accountSetupSteps/PersonalStep.vue';
import SuccessStep from '@/components/dialogs/accountSetupSteps/SuccessStep.vue';
import PricingPlanSelectionStep from '@/components/dialogs/upgradeAccountFlow/PricingPlanSelectionStep.vue';
import PricingPlanStep from '@/components/dialogs/upgradeAccountFlow/PricingPlanStep.vue';
import ManagedPassphraseOptInStep from '@/components/dialogs/accountSetupSteps/ManagedPassphraseOptInStep.vue';
import AccountTypeStep from '@/components/dialogs/accountSetupSteps/AccountTypeStep.vue';
import IconStorjLogo from '@/components/icons/IconStorjLogo.vue';
import AddTokensStep from '@/components/dialogs/upgradeAccountFlow/AddTokensStep.vue';

type SetupLocation = OnboardingStep | undefined | (() => (OnboardingStep | undefined));
interface SetupStep {
    setup?: () => void | Promise<void>;
    validate?: () => boolean;
}

class StepInfo {
    public ref = ref<SetupStep>();
    public prev: Ref<OnboardingStep | undefined>;
    public next: Ref<OnboardingStep | undefined>;

    constructor(
        prev: SetupLocation = undefined,
        next: SetupLocation = undefined,
        public beforeNext?: () => Promise<void>,
    ) {
        this.prev = (typeof prev === 'function') ? computed<OnboardingStep | undefined>(prev) : ref<OnboardingStep | undefined>(prev);
        this.next = (typeof next === 'function') ? computed<OnboardingStep | undefined>(next) : ref<OnboardingStep | undefined>(next);
    }
}

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const billingStore = useBillingStore();
const configStore = useConfigStore();
const projectsStore = useProjectsStore();
const userStore = useUsersStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const stepInfos = {
    [OnboardingStep.AccountTypeSelection]: new StepInfo(
        undefined,
        () => accountType.value,
        async () => {
            const update: SetUserSettingsData = { onboardingStep: accountType.value };
            if (!userSettings.value.onboardingStart) {
                update.onboardingStart = true;
            }
            await userStore.updateSettings(update);
        },
    ),
    [OnboardingStep.BusinessAccountForm]: (() => {
        const info = new StepInfo(
            OnboardingStep.AccountTypeSelection,
            () => {
                if (pkgAvailable.value) return OnboardingStep.PricingPlanSelection;
                return OnboardingStep.PlanTypeSelection;
            },
        );
        info.beforeNext =  async () => {
            await Promise.all([
                userStore.updateSettings({ onboardingStep: info.next.value }),
                info.ref.value?.setup?.(),
            ]);
        };
        return info;
    })(),
    [OnboardingStep.PersonalAccountForm]: (() => {
        const info = new StepInfo(
            OnboardingStep.AccountTypeSelection,
            () => {
                if (pkgAvailable.value) return OnboardingStep.PricingPlanSelection;
                return OnboardingStep.PlanTypeSelection;
            },
        );
        info.beforeNext =  async () => {
            await Promise.all([
                userStore.updateSettings({ onboardingStep: info.next.value }),
                info.ref.value?.setup?.(),
            ]);
        };
        return info;
    })(),
    [OnboardingStep.PricingPlanSelection]: new StepInfo(
        () => accountType.value || OnboardingStep.AccountTypeSelection,
        OnboardingStep.PricingPlan,
    ),
    [OnboardingStep.PlanTypeSelection]: new StepInfo(
        () => accountType.value || OnboardingStep.AccountTypeSelection,
        () => plan.value ? OnboardingStep.PricingPlan : OnboardingStep.PaymentMethodSelection,
    ),
    [OnboardingStep.PaymentMethodSelection]: new StepInfo(
        OnboardingStep.PlanTypeSelection,
        () => plan.value ? OnboardingStep.PricingPlan : OnboardingStep.AddTokens,
    ),
    [OnboardingStep.AddTokens]: (() => {
        const info = new StepInfo(
            OnboardingStep.PaymentMethodSelection,
            () => {
                if (allowManagedPassphraseStep.value) return OnboardingStep.ManagedPassphraseOptIn;
                return OnboardingStep.SetupComplete;
            },
        );
        info.beforeNext =  async () => {
            await userStore.updateSettings({ onboardingStep: info.next.value });
        };
        return info;
    })(),
    [OnboardingStep.PricingPlan]: (() => {
        const info = new StepInfo(
            () => {
                if (pkgAvailable.value) return OnboardingStep.PricingPlanSelection;
                if (plan.value?.type === PricingPlanType.FREE) return  OnboardingStep.PlanTypeSelection;
                return OnboardingStep.PaymentMethodSelection;
            },
            () => {
                if (allowManagedPassphraseStep.value) return OnboardingStep.ManagedPassphraseOptIn;
                return OnboardingStep.SetupComplete;
            },
        );
        info.beforeNext =  async () => {
            await userStore.updateSettings({ onboardingStep: info.next.value });
        };
        return info;
    })(),
    [OnboardingStep.ManagedPassphraseOptIn]: (() => {
        const info = new StepInfo(
            undefined,
            OnboardingStep.SetupComplete,
        );
        info.beforeNext =  async () => {
            await Promise.all([
                userStore.updateSettings({ onboardingStep: info.next.value }),
                info.ref.value?.setup?.(),
            ]);
        };
        return info;
    })(),
    [OnboardingStep.SetupComplete]: new StepInfo(
        undefined,
        undefined,
        async () => {
            await stepInfos[OnboardingStep.SetupComplete].ref.value?.setup?.();
        },
    ),
};

const innerContent = ref<Component | null>(null);
const step = ref<OnboardingStep>(OnboardingStep.AccountTypeSelection);
const plan = ref<PricingPlanInfo>();
const passphraseManageMode = ref<ManagePassphraseMode>('auto');
const accountType = ref<OnboardingStep>();

const firstName = ref<string>('');
const lastName = ref<string>('');
const companyName = ref<string>('');
const position = ref<string | undefined>(undefined);
const employeeCount = ref<string | undefined>(undefined);
const storageNeeds = ref<string | undefined>(undefined);
const useCase = ref<string | undefined>(undefined);
const otherUseCase = ref<string | undefined>(undefined);
const functionalArea = ref<string | undefined>(undefined);
const haveSalesContact = ref<boolean>(false);
const interestedInPartnering = ref<boolean>(false);

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

function onChoiceSelect(s: OnboardingStep) {
    accountType.value = s;
    toNextStep();
}

function onSelectPricingPlan(p: PricingPlanInfo) {
    plan.value = p;
    toNextStep();
}

/**
 * Claims wallet and sets add token step.
 */
function onAddTokens() {
    withLoading(async () => {
        try {
            await new Promise((r) => setTimeout(r, 3000));
            await billingStore.claimWallet();
            analyticsStore.eventTriggered(AnalyticsEvent.ADD_FUNDS_CLICKED);
            toNextStep();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETUP_DIALOG);
        }
    });
}

/**
 * Decides whether to move to the success step or the pricing plan selection.
 */
async function toNextStep() {
    const info = stepInfos[step.value];
    if (info.ref.value?.validate?.() === false) {
        return;
    }

    isLoading.value = true;
    try {
        await info.beforeNext?.();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETUP_DIALOG);
        return;
    } finally {
        isLoading.value = false;
    }
    if (info.next.value) {
        step.value = info.next.value;
    }
}

async function toPrevStep() {
    const info = stepInfos[step.value];
    if (info.prev.value) {
        step.value = info.prev.value;
    }
    plan.value = undefined;
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
            step.value = allowManagedPassphraseStep.value ? OnboardingStep.ManagedPassphraseOptIn : OnboardingStep.SetupComplete;
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
