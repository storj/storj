// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p v-if="isPaidTier" class="text-body-2 mb-4">
        Send STORJ Tokens to the following deposit address to credit your Storj account. In addition, you will receive a 10% bonus on your deposit.
    </p>
    <p v-else class="text-body-2 mb-4">
        Send more than $10 in STORJ Tokens to the following deposit address to upgrade to a Pro account.
        Your account will be upgraded after your transaction receives {{ neededConfirmations }} confirmations.
        If your account is not automatically upgraded, please fill out this
        <a
            href="https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000683212"
            target="_blank"
            class="link"
            rel="noopener noreferrer"
        >limit increase request form</a>.
    </p>

    <v-row class="ma-0 border rounded-lg" justify="center">
        <v-col cols="auto">
            <canvas ref="canvas" />
        </v-col>
    </v-row>

    <p class="text-caption font-weight-bold my-3">
        Deposit Address
        <v-tooltip max-width="200px" location="top">
            <template #activator="{ props }">
                <v-icon v-bind="props" :icon="Info" size="16" />
            </template>
            <p>
                This is a Storj token deposit address generated just for you.
                <a
                    href="https://docs.storj.io/support/account-management-billing/billing#adding-storj-tokens"
                    class="link"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    Learn more
                </a>
            </p>
        </v-tooltip>
    </p>

    <v-row justify="space-between" align="center" class="ma-0 mb-4 border-sm rounded-lg">
        <v-col>
            <p class="text-caption">{{ wallet.address }}</p>
        </v-col>

        <v-col cols="auto">
            <v-btn
                density="compact"
                :prepend-icon="Copy"
                @click="onCopyAddressClick"
            >
                Copy
            </v-btn>
        </v-col>
    </v-row>

    <AddTokensStepBanner
        :is-default="viewState === ViewState.Default"
        :is-pending="viewState === ViewState.Pending"
        :is-success="viewState === ViewState.Success"
        :pending-payments="pendingPayments"
    />

    <template v-if="!isRoot">
        <v-alert
            class="mt-3 mb-2"
            density="compact"
            variant="tonal"
            type="info"
            text="You can continue using the app, and your upgrade will be applied automatically once your STORJ tokens are received."
        />
        <v-card-actions class="px-0">
            <v-row class="ma-0 gap">
                <v-col v-if="viewState !== ViewState.Success" class="px-0">
                    <v-btn
                        block
                        variant="outlined"
                        color="default"
                        @click="() => emit('back')"
                    >
                        Back
                    </v-btn>
                </v-col>
                <v-col v-if="isOnboarding" class="px-0">
                    <v-btn
                        color="primary"
                        variant="flat"
                        block
                        @click="() => emit('success')"
                    >
                        Next
                    </v-btn>
                </v-col>
            </v-row>
        </v-card-actions>
    </template>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import QRCode from 'qrcode';
import { VTooltip, VBtn, VIcon, VCol, VRow, VCardActions, VAlert } from 'vuetify/components';
import { Copy, Info } from 'lucide-vue-next';

import { useBillingStore } from '@/store/modules/billingStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/utils/hooks';
import { PaymentStatus, PaymentWithConfirmations, Wallet } from '@/types/payments';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import AddTokensStepBanner from '@/components/dialogs/upgradeAccountFlow/AddTokensStepBanner.vue';

enum ViewState {
    Default,
    Pending,
    Success,
}

const configStore = useConfigStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const notify = useNotify();

const canvas = ref<HTMLCanvasElement>();
const viewState = ref<ViewState>(ViewState.Default);

defineProps<{
    // whether this step is the first step in a flow
    isRoot?: boolean;
    isOnboarding?: boolean;
}>();

const emit = defineEmits<{
    back: [];
    success: [];
}>();

/**
 * Returns wallet from store.
 */
const wallet = computed((): Wallet => {
    return billingStore.state.wallet as Wallet;
});

/**
 * Returns needed transaction confirmations from config store.
 */
const neededConfirmations = computed((): number => {
    return configStore.state.config.neededTransactionConfirmations;
});

/**
 * Returns pending payments from store.
 */
const pendingPayments = computed((): PaymentWithConfirmations[] => {
    return billingStore.state.pendingPaymentsWithConfirmations;
});

/**
 * Returns whether the user is in paid tier.
 */
const isPaidTier = computed((): boolean => usersStore.state.user.paidTier);

/**
 * Copies address to user's clipboard.
 */
function onCopyAddressClick(): void {
    navigator.clipboard.writeText(wallet.value.address);
    notify.success('Address copied to your clipboard');
}

/**
 * Sets current view state depending on payment statuses.
 */
function setViewState(): void {
    switch (true) {
    case pendingPayments.value.some(p => p.status === PaymentStatus.Pending):
        viewState.value = ViewState.Pending;
        break;
    case pendingPayments.value.some(p => p.status === PaymentStatus.Confirmed):
        viewState.value = ViewState.Success;
        break;
    default:
        viewState.value = ViewState.Default;
    }
}

watch(isPaidTier, newVal => {
    if (newVal) {
        // arbitrary delay to allow user to read success banner.
        setTimeout(() => emit('success'), 2000);
    }
});

watch(() => pendingPayments.value, async () => {
    setViewState();
}, { deep: true });

/**
 * Mounted lifecycle hook after initial render.
 * Renders QR code.
 */
onMounted(async (): Promise<void> => {
    setViewState();

    if (!isPaidTier.value) {
        billingStore.startPaymentsPolling();
    }

    if (!canvas.value) {
        return;
    }

    try {
        await QRCode.toCanvas(canvas.value, wallet.value.address, { width: 124 });
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
    }
});
</script>

<style scoped lang="scss">
.gap {
    column-gap: 12px;
}
</style>
