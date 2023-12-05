// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <template v-if="!isSuccess">
        <v-row class="ma-0" justify="space-between" align="center">
            <v-col class="px-0" cols="auto">
                <span class="font-weight-bold">Activate your plan</span>
            </v-col>
            <v-col class="px-0" cols="auto">
                <v-btn density="compact" color="success" variant="tonal" icon>
                    <v-icon icon="mdi-check-outline" />
                </v-btn>
            </v-col>
        </v-row>

        <v-row class="ma-0" align="center">
            <v-col class="px-0" cols="9">
                <div class="pt-4">
                    <p class="font-weight-bold">{{ plan.title }} <span v-if="plan.activationSubtitle"> / {{ plan.activationSubtitle }}</span></p>
                </div>

                <div>
                    <!-- eslint-disable-next-line vue/no-v-html -->
                    <p v-html="plan.activationDescriptionHTML" />
                </div>
            </v-col>
            <v-col v-if="plan.activationPriceHTML" class="px-0" cols="3">
                <!-- eslint-disable-next-line vue/no-v-html -->
                <p class="font-weight-bold" v-html="plan.activationPriceHTML" />
            </v-col>
        </v-row>

        <div v-if="!isFree" class="py-4">
            <p class="text-caption">Add Card Info</p>
            <StripeCardElement
                v-if="paymentElementEnabled"
                ref="stripeCardInput"
                :is-dark-theme="theme.global.current.value.dark"
                @pm-created="onCardAdded"
            />
            <StripeCardInput
                v-else
                ref="stripeCardInput"
                :on-stripe-response-callback="onCardAdded"
            />
        </div>

        <div class="py-4">
            <v-btn
                block
                :color="plan.type === 'partner' ? 'success' : 'primary'"
                :loading="isLoading"
                @click="onActivateClick"
            >
                <template v-if="plan.type !== 'free'" #prepend>
                    <v-icon icon="mdi-lock" />
                </template>

                {{ plan.activationButtonText || ('Activate ' + plan.title) }}
            </v-btn>
        </div>

        <div class="pb-4">
            <v-btn
                block
                variant="outlined"
                color="grey-lighten-1"
                :disabled="isLoading"
                @click="emit('back')"
            >
                Back
            </v-btn>
        </div>
    </template>

    <template v-else>
        <v-row class="ma-0" justify="center" align="center">
            <v-col cols="auto">
                <v-btn density="comfortable" color="success" variant="tonal" icon>
                    <v-icon icon="mdi-check-outline" />
                </v-btn>
            </v-col>
        </v-row>

        <h1 class="text-center">Success</h1>

        <p class="text-center mb-4">Your plan has been successfully activated.</p>

        <v-row align="center" justify="space-between" class="ma-0 mb-4 pa-2 border-sm rounded-lg">
            <v-col cols="auto">
                <v-icon color="success" icon="mdi-check-outline" />
            </v-col>
            <v-col cols="auto">
                <span class="text-body-1 font-weight-bold">
                    {{ plan.title }}
                    <span v-if="plan.activationSubtitle" class="font-weight-regular"> / {{ plan.successSubtitle }}</span>
                </span>
            </v-col>
            <v-col cols="auto">
                <span style="color: var(--c-green-5);">Activated</span>
            </v-col>
        </v-row>

        <v-btn
            color="success"
            block
            @click="emit('close')"
        >
            Continue
        </v-btn>
    </template>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import { VBtn, VCol, VIcon, VRow } from 'vuetify/components';
import { useTheme } from 'vuetify';

import { PricingPlanInfo, PricingPlanType } from '@/types/common';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useConfigStore } from '@/store/modules/configStore';

import StripeCardElement from '@/components/account/billing/paymentMethods/StripeCardElement.vue';
import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';

interface StripeForm {
    onSubmit(): Promise<void>;
}

const configStore = useConfigStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const router = useRouter();
const notify = useNotify();
const theme = useTheme();

const isLoading = ref<boolean>(false);
const isSuccess = ref<boolean>(false);

const stripeCardInput = ref<StripeForm | null>(null);

const props = withDefaults(defineProps<{
    plan?: PricingPlanInfo;
}>(), {
    plan: () => new PricingPlanInfo(),
});

const emit = defineEmits<{
    back: [];
    close: [];
    success: []; // emit this for parents that have custom success steps.
}>();

/**
 * Indicates whether stripe payment element is enabled.
 */
const paymentElementEnabled = computed(() => {
    return configStore.state.config.stripePaymentElementEnabled;
});

/**
 * Returns whether current plan is a free pricing plan.
 */
const isFree = computed((): boolean => {
    return props.plan?.type === PricingPlanType.FREE;
});

/**
 * Applies the selected pricing plan to the user.
 */
async function onActivateClick() {
    if (isLoading.value || !props.plan) return;
    isLoading.value = true;

    if (isFree.value) {
        emit('success');
        isSuccess.value = true;
        return;
    }

    try {
        await stripeCardInput.value?.onSubmit();
    } catch (error) {
        notify.notifyError(error, null);
    } finally {
        isLoading.value = false;
    }
}

/**
 * Adds card after Stripe confirmation.
 * @param res - the response from stripe. Could be a token or a payment method id.
 * depending on the paymentElementEnabled flag.
 */
async function onCardAdded(res: string): Promise<void> {
    if (!props.plan) return;
    isLoading.value = true;
    try {
        if (props.plan.type === PricingPlanType.PARTNER) {
            await billingStore.purchasePricingPackage(res, paymentElementEnabled.value);
        } else {
            paymentElementEnabled.value ? await billingStore.addCardByPaymentMethodID(res) : await billingStore.addCreditCard(res);
        }
        emit('success');
        isSuccess.value = true;

        // Fetch user to update paid tier status
        await usersStore.getUser();
        // Fetch cards to hide paid tier banner
        await billingStore.getCreditCards();
    } catch (error) {
        notify.notifyError(error, null);
    }

    isLoading.value = false;
}
</script>