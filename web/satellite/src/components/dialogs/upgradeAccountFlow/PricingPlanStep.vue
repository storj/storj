// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <template v-if="!isSuccess">
        <v-row class="ma-0" align="center">
            <v-col class="px-0">
                <p class="font-weight-bold">{{ plan.title }} <span v-if="plan.activationSubtitle"> / {{ plan.activationSubtitle }}</span></p>
                <!-- eslint-disable-next-line vue/no-v-html -->
                <p v-html="plan.activationDescriptionHTML" />
                <!-- eslint-disable-next-line vue/no-v-html -->
                <v-chip v-if="plan.activationPriceHTML" color="success" :prepend-icon="Check" class="mt-2"><p class="font-weight-bold" v-html="plan.activationPriceHTML" /></v-chip>
            </v-col>
        </v-row>

        <div v-if="!isFree" class="my-4">
            <p class="text-caption mb-2">Add Card Info</p>
            <StripeCardElement
                v-if="paymentElementEnabled"
                ref="stripeCardInput"
                @ready="stripeReady = true"
            />
            <StripeCardInput
                v-else
                ref="stripeCardInput"
                @ready="stripeReady = true"
            />
        </div>

        <div class="py-4">
            <v-btn
                id="activate"
                block
                :color="plan.type === 'partner' ? 'success' : 'primary'"
                :disabled="!stripeReady && !isFree"
                :loading="loading"
                @click="onActivateClick"
            >
                <template v-if="plan.type !== 'free'" #prepend>
                    <v-icon :icon="LockKeyhole" />
                </template>

                {{ plan.activationButtonText || ('Activate ' + plan.title) }}
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
        <v-row class="ma-0" justify="center" align="center">
            <v-col cols="auto">
                <v-btn density="comfortable" color="success" variant="tonal" icon>
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
                    {{ plan.title }}
                </p>
                <p v-if="plan.activationSubtitle">{{ plan.activationSubtitle }}</p>
            </template>
            <template #append>
                <span>Activated</span>
            </template>
        </v-alert>

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
import { VAlert, VBtn, VCol, VIcon, VRow, VChip } from 'vuetify/components';
import { Check, LockKeyhole, ChevronLeft } from 'lucide-vue-next';

import { PricingPlanInfo, PricingPlanType } from '@/types/common';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useConfigStore } from '@/store/modules/configStore';

import StripeCardElement from '@/components/StripeCardElement.vue';
import StripeCardInput from '@/components/StripeCardInput.vue';

interface StripeForm {
    onSubmit(): Promise<string>;
}

const configStore = useConfigStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();

const notify = useNotify();

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
        notify.notifyError(error);
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
    try {
        if (props.plan.type === PricingPlanType.PARTNER) {
            await billingStore.purchasePricingPackage(res, paymentElementEnabled.value);
        } else {
            if (paymentElementEnabled.value) {
                await billingStore.addCardByPaymentMethodID(res);
            } else {
                await billingStore.addCreditCard(res);
            }
        }
        onSuccess();

        // Fetch user to update paid tier status
        usersStore.getUser().catch((_) => {});
        // Fetch cards to hide paid tier banner
        billingStore.getCreditCards().catch((_) => {});
    } catch (error) {
        notify.notifyError(error);
    }
}

function onSuccess() {
    if (props.isAccountSetup) {
        emit('success');
    } else {
        isSuccess.value = true;
    }
}
</script>
