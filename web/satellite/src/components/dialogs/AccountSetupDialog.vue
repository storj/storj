// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog :model-value="shouldShowSetupDialog" fullscreen persistent transition="fade-transition" scrollable>
        <v-card class="account-setup-dialog">
            <v-card-item class="pa-1" :class="{ 'h-100': step === OnboardingStep.SetupComplete }">
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

                    <template v-if="billingEnabled">
                        <v-window-item :value="OnboardingStep.PlanTypeSelection">
                            <account-type-step
                                @free-click="() => onSelectPricingPlan(FREE_PLAN_INFO)"
                                @pro-click="() => onSelectPricingPlan(PRO_PLAN_INFO)"
                                @back="toPrevStep"
                            />
                        </v-window-item>

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
                                <v-window v-model="paymentTab">
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

                    <!-- Final step -->
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
    Ref,
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
    ACCOUNT_SETUP_STEPS,
    OnboardingStep,
    SetUserSettingsData,
    UserSettings,
} from '@/types/users';
import { FREE_PLAN_INFO, PricingPlanInfo, PricingPlanType, PRO_PLAN_INFO } from '@/types/common';
import { useConfigStore } from '@/store/modules/configStore';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { ManagePassphraseMode } from '@/types/projects';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { Wallet } from '@/types/payments';

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

enum PaymentOption {
    CreditCard,
    StorjTokens,
}

class StepInfo {
    public ref = ref<SetupStep>();
    public prev: Ref<OnboardingStep | undefined>;
    public next: Ref<OnboardingStep | undefined>;

    constructor(
        prev: SetupLocation = undefined,
        next: SetupLocation = undefined,
        public beforeNext?: () => void | Promise<void>,
    ) {
        this.prev = (typeof prev === 'function') ? computed<OnboardingStep | undefined>(prev) : ref<OnboardingStep | undefined>(prev);
        this.next = (typeof next === 'function') ? computed<OnboardingStep | undefined>(next) : ref<OnboardingStep | undefined>(next);
    }
}

const analyticsStore = useAnalyticsStore();
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
                if (!billingEnabled.value) {
                    if (allowManagedPassphraseStep.value) return OnboardingStep.ManagedPassphraseOptIn;
                    return OnboardingStep.SetupComplete;
                }
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
                if (!billingEnabled.value) {
                    if (allowManagedPassphraseStep.value) return OnboardingStep.ManagedPassphraseOptIn;
                    return OnboardingStep.SetupComplete;
                }
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
        () => accountType.value ?? OnboardingStep.AccountTypeSelection,
        () => {
            if (!isFreePlan.value) return OnboardingStep.PaymentMethodSelection;
            return allowManagedPassphraseStep.value ? OnboardingStep.ManagedPassphraseOptIn : OnboardingStep.SetupComplete;
        },
        async () => {
            if (isFreePlan.value) {
                await userStore.updateSettings({
                    onboardingStep: allowManagedPassphraseStep.value ? OnboardingStep.ManagedPassphraseOptIn : OnboardingStep.SetupComplete,
                });
            }
        },
    ),
    [OnboardingStep.PlanTypeSelection]: new StepInfo(
        () => accountType.value ?? OnboardingStep.AccountTypeSelection,
        () => {
            if (!isFreePlan.value) return OnboardingStep.PaymentMethodSelection;
            return allowManagedPassphraseStep.value ? OnboardingStep.ManagedPassphraseOptIn : OnboardingStep.SetupComplete;
        },
        async () => {
            if (isFreePlan.value) {
                await userStore.updateSettings({
                    onboardingStep: allowManagedPassphraseStep.value ? OnboardingStep.ManagedPassphraseOptIn : OnboardingStep.SetupComplete,
                });
            }
        },
    ),
    [OnboardingStep.PaymentMethodSelection]: new StepInfo(
        () => pkgAvailable.value ? OnboardingStep.PricingPlanSelection : OnboardingStep.PlanTypeSelection,
        () => allowManagedPassphraseStep.value ? OnboardingStep.ManagedPassphraseOptIn : OnboardingStep.SetupComplete,
        async () => {
            await userStore.updateSettings({
                onboardingStep: allowManagedPassphraseStep.value ? OnboardingStep.ManagedPassphraseOptIn : OnboardingStep.SetupComplete,
            });
        },
    ),
    [OnboardingStep.ManagedPassphraseOptIn]: (() => {
        const info = new StepInfo(
            undefined,
            OnboardingStep.SetupComplete,
        );
        info.beforeNext =  async () => {
            await info.ref.value?.setup?.();
            await userStore.updateSettings({ onboardingStep: info.next.value });
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

const step = ref<OnboardingStep>(OnboardingStep.AccountTypeSelection);
const plan = ref<PricingPlanInfo>();
const passphraseManageMode = ref<ManagePassphraseMode>('auto');
const accountType = ref<OnboardingStep.BusinessAccountForm | OnboardingStep.PersonalAccountForm>();
const paymentTab = ref<PaymentOption>(PaymentOption.CreditCard);

const isAccountSetup = ref<boolean>(false);
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

const billingEnabled = computed<boolean>(() => configStore.state.config.billingFeaturesEnabled);

const pkgAvailable = computed<boolean>(() => billingStore.state.pricingPlansAvailable);

const isProPlan = computed<boolean>(() => plan.value?.type === PricingPlanType.PRO);

const isFreePlan = computed<boolean>(() => plan.value?.type === PricingPlanType.FREE);

const wallet = computed<Wallet>(() => billingStore.state.wallet as Wallet);

/**
 * Indicates if satellite managed encryption passphrase is enabled.
 */
const satelliteManagedEncryptionEnabled = computed<boolean>(() => configStore.state.config.satelliteManagedEncryptionEnabled);

/**
 * Indicates whether to allow progression to managed passphrase opt in step.
 */
const allowManagedPassphraseStep = computed<boolean>(() => satelliteManagedEncryptionEnabled.value && projectsStore.state.projects.length === 0);

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

function onChoiceSelect(s: OnboardingStep.BusinessAccountForm | OnboardingStep.PersonalAccountForm): void {
    accountType.value = s;
    toNextStep();
}

function onSelectPricingPlan(p: PricingPlanInfo): void {
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
            analyticsStore.eventTriggered(AnalyticsEvent.ADD_FUNDS_CLICKED);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETUP_DIALOG);
        }
    });
}

/**
 * Decides whether to move to the success step or the pricing plan selection.
 */
async function toNextStep(): Promise<void> {
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

async function toPrevStep(): Promise<void> {
    const info = stepInfos[step.value];
    if (info.prev.value) {
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

    firstName.value = userStore.userName || '';

    switch (true) {
    case currentStep === OnboardingStep.SetupComplete ||
        (currentStep === OnboardingStep.ManagedPassphraseOptIn && !allowManagedPassphraseStep.value):
        step.value = OnboardingStep.SetupComplete;
        break;
    case currentStep === OnboardingStep.PricingPlanSelection && !pkgAvailable.value:
        step.value = allowManagedPassphraseStep.value ? OnboardingStep.ManagedPassphraseOptIn : OnboardingStep.SetupComplete;
        break;
    case ACCOUNT_SETUP_STEPS.some(s => s === currentStep):
        step.value = currentStep as OnboardingStep;
        break;
    case !userStore.userName:
        step.value = OnboardingStep.AccountTypeSelection;
        break;
    case pkgAvailable.value:
        step.value = OnboardingStep.PricingPlanSelection;
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
