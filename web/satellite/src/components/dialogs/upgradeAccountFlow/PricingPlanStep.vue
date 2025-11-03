// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <template v-if="!isSuccess">
        <v-sheet elevation="0" border="sm" rounded="lg" color="background" class="pa-5 mb-4">
            <template v-if="plan.planUpfrontCharge">
                Add {{ plan.planUpfrontCharge }} to activate your account - this stays as your account balance.
                <br>
            </template>
            <!-- eslint-disable-next-line vue/no-v-html -->
            <p v-if="plan.planMinimumFeeInfo" v-html="plan.planMinimumFeeInfo" />

            <div v-if="plan.planUpfrontCharge" class="d-flex align-center justify-start mt-4 ga-5">
                <v-sheet border="sm" elevation="0" rounded="lg" class="py-1 px-3 custom-border">
                    <span class="text-body-1 font-weight-bold"> Total Today: {{ plan.planUpfrontCharge || '$0' }} </span>
                </v-sheet>
                <span v-if="plan.planUpfrontCharge" class="text-body-1 font-weight-bold">
                    <v-icon :icon="Check" /> {{ plan.planBalanceCredit }} will be added to your account balance</span>
            </div>
        </v-sheet>

        <div v-if="!isFree" class="my-2">
            <StripeCardElement
                ref="stripeCardInput"
                @ready="stripeReady = true"
            />
        </div>

        <template v-if="isAccountSetup">
            <div class="py-4">
                <v-btn
                    id="activate"
                    block
                    :color="plan.type === 'partner' ? 'secondary' : 'primary'"
                    :disabled="!stripeReady && !isFree"
                    :loading="loading"
                    @click="onActivateClick"
                >
                    {{ plan.activationButtonText || ('Activate ' + plan.planTitle) }}
                </v-btn>
            </div>

            <div class="pb-4">
                <v-btn
                    block
                    variant="text"
                    color="default"
                    :prepend-icon="ChevronLeft"
                    :disabled="loading"
                    @click="onBack"
                >
                    Back
                </v-btn>
            </div>
        </template>
        <template v-else>
            <v-row justify="center" class="mx-0 mt-2 mb-1">
                <v-col class="pl-0">
                    <v-btn
                        block
                        variant="outlined"
                        color="default"
                        :disabled="loading"
                        @click="onBack"
                    >
                        Back
                    </v-btn>
                </v-col>
                <v-col class="px-0">
                    <v-btn
                        id="activate"
                        block
                        :color="plan.type === 'partner' ? 'secondary' : 'primary'"
                        :disabled="!stripeReady && !isFree"
                        :loading="loading"
                        @click="onActivateClick"
                    >
                        {{ plan.activationButtonText || ('Activate ' + plan.planTitle) }}
                    </v-btn>
                </v-col>
            </v-row>
        </template>
    </template>

    <template v-else>
        <v-row class="ma-0" justify="center" align="center">
            <v-col cols="auto">
                <v-btn density="comfortable" variant="tonal" icon>
                    <v-icon :icon="Check" />
                </v-btn>
            </v-col>
        </v-row>

        <h1 class="text-center">Success</h1>

        <p class="text-center mb-4">Your plan has been successfully activated.</p>

        <v-alert
            class="mb-4"
            type="success"
            variant="tonal"
        >
            <template #prepend>
                <v-icon :icon="Check" />
            </template>
            <template #text>
                <p class="font-weight-bold">
                    {{ plan.planTitle }}
                </p>
                <p v-if="plan.activationSubtitle">{{ plan.activationSubtitle }}</p>
            </template>
            <template #append>
                <span>Activated</span>
            </template>
        </v-alert>

        <v-btn
            block
            @click="emit('close')"
        >
            Continue
        </v-btn>
    </template>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VAlert, VBtn, VCol, VIcon, VRow, VSheet } from 'vuetify/components';
import { Check, ChevronLeft } from 'lucide-vue-next';
import { useRoute } from 'vue-router';

import { PricingPlanInfo, PricingPlanType } from '@/types/common';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useConfigStore } from '@/store/modules/configStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { ROUTES } from '@/router';
import { useProjectsStore } from '@/store/modules/projectsStore';

import StripeCardElement from '@/components/StripeCardElement.vue';

interface StripeForm {
    onSubmit(): Promise<string>;
    initStripe(): Promise<string>;
}

const analyticsStore = useAnalyticsStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();
const projectsStore = useProjectsStore();

const notify = useNotify();
const route = useRoute();

const isSuccess = ref<boolean>(false);

const stripeCardInput = ref<StripeForm | null>(null);
const stripeReady = ref<boolean>(false);

const props = withDefaults(defineProps<{
    plan?: PricingPlanInfo;
    isAccountSetup?: boolean;
}>(), {
    plan: () => new PricingPlanInfo(),
    isAccountSetup: false,
});

const emit = defineEmits<{
    back: [];
    close: [];
    success: []; // emit this for parents that have custom success steps.
}>();

const loading = defineModel<boolean>('loading');

/**
 * Returns whether current plan is a free pricing plan.
 */
const isFree = computed<boolean>(() => props.plan?.type === PricingPlanType.FREE);

const upgradePayUpfrontAmount = computed(() => configStore.state.config.upgradePayUpfrontAmount);

function onBack(): void {
    stripeReady.value = false;
    emit('back');
}

/**
 * Applies the selected pricing plan to the user.
 */
async function onActivateClick() {
    if (loading.value || !props.plan) return;

    if (isFree.value) {
        onSuccess();
        return;
    }

    if (!stripeCardInput.value) return;

    loading.value = true;
    try {
        const response = await stripeCardInput.value.onSubmit();
        await onCardAdded(response);
    } catch (error) {
        const source = props.isAccountSetup ? AnalyticsErrorEventSource.ACCOUNT_SETUP_DIALOG : AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL;
        notify.notifyError(error, source);

        // initStripe will get a new card setup secret if there's an error.
        stripeCardInput.value?.initStripe();
    }
    loading.value = false;
}

/**
 * Adds card after Stripe confirmation.
 * @param res - the response from stripe. Could be a token or a payment method id.
 * depending on the paymentElementEnabled flag.
 */
async function onCardAdded(res: string): Promise<void> {
    if (!props.plan) return;

    if (props.plan.type === PricingPlanType.PARTNER) {
        await billingStore.purchasePricingPackage(res);
    } else {
        if (upgradePayUpfrontAmount.value > 0) {
            await billingStore.purchaseUpgradedAccount(res);
        } else {
            await billingStore.addCardByPaymentMethodID(res);
        }
    }
    onSuccess();

    // We fetch User one more time to update their Paid Tier status.
    usersStore.getUser().catch((_) => {});

    if (
        route.name === ROUTES.Dashboard.name ||
        route.name === ROUTES.Domains.name ||
        route.name === ROUTES.Buckets.name ||
        route.name === ROUTES.Bucket.name
    ) {
        Promise.all([
            projectsStore.getProjectConfig(),
            projectsStore.getProjectLimits(projectsStore.state.selectedProject.id),
        ]).catch(_ => {});
    }

    if (route.name === ROUTES.Billing.name) {
        billingStore.getCreditCards().catch((_) => {});
    }
}

function onSuccess() {
    analyticsStore.eventTriggered(AnalyticsEvent.MODAL_ADD_CARD);
    loading.value = false;
    notify.success('Card successfully added and account upgraded');

    if (props.isAccountSetup || props.plan.type !== PricingPlanType.PARTNER) {
        emit('success');
    } else {
        isSuccess.value = true;
    }
}
</script>

<style scoped lang="scss">
.custom-border {
    border-color: currentcolor !important;
}
</style>