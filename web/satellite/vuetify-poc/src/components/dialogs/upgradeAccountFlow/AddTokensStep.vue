// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p class="text-body-2 mb-4">
        Send more than $10 in STORJ Tokens to the following deposit address to upgrade to a Pro account.
        Your account will be upgraded after your transaction receives {{ neededConfirmations }} confirmations.
        If your account is not automatically upgraded, please fill out this
        <a
            style="color: var(--c-blue-3);"
            href="https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000683212"
            target="_blank"
            rel="noopener noreferrer"
        >limit increase request form</a>.
    </p>

    <v-row class="ma-0 border rounded-lg" justify="center">
        <v-col cols="auto">
            <canvas ref="canvas" />
        </v-col>
    </v-row>

    <p class="text-caption font-weight-bold">
        Deposit Address
        <v-tooltip max-width="200px" location="top">
            <template #activator="{ props }">
                <v-btn v-bind="props" density="compact" variant="plain" color="grey" icon>
                    <v-icon icon="mdi-information-outline" size="16" />
                </v-btn>
            </template>
            <p>
                This is a Storj token deposit address generated just for you.
                <a
                    style="color: var(--c-white);"
                    href=""
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    Learn more
                </a>
            </p>
        </v-tooltip>
    </p>

    <v-row justify="space-between" align="center" class="ma-0 mb-4 border-sm rounded-lg">
        <v-col class="pb-0">
            <p class="text-caption">{{ wallet.address }}</p>
        </v-col>

        <v-col class="pa-2 pr-3" cols="auto">
            <v-btn
                density="compact"
                @click="onCopyAddressClick"
            >
                <template #prepend>
                    <v-icon icon="mdi-content-copy" />
                </template>
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

    <v-btn
        v-if="viewState !== ViewState.Success && !isRoot"
        class="my-4"
        block
        variant="outlined"
        color="default"
        @click="emit('back')"
    >
        Back
    </v-btn>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import QRCode from 'qrcode';
import { VTooltip, VBtn, VIcon, VCol, VRow, VDivider } from 'vuetify/components';

import { useBillingStore } from '@/store/modules/billingStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useNotify } from '@/utils/hooks';
import { PaymentStatus, PaymentWithConfirmations, Wallet } from '@/types/payments';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useUsersStore } from '@/store/modules/usersStore';

import AddTokensStepBanner from '@poc/components/dialogs/upgradeAccountFlow/AddTokensStepBanner.vue';

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
const intervalID = ref<NodeJS.Timer>();
const viewState = ref<ViewState>(ViewState.Default);

const props = defineProps<{
    // whether this step is the first step in a flow
    isRoot?: boolean;
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

watch(() => pendingPayments.value, async () => {
    setViewState();

    if (viewState.value !== ViewState.Success) {
        return;
    }
    clearInterval(intervalID.value);
    billingStore.clearPendingPayments();

    if (usersStore.state.user.paidTier) {
        // in case this step was entered in to directly from
        // the billing/payment method tab when the user is
        // already in paid tier.
        return;
    }

    // fetch User to update their Paid Tier status.
    await usersStore.getUser();

    // arbitrary delay to allow for user to read success banner.
    await new Promise(resolve => setTimeout(resolve, 2000));
    emit('success');
}, { deep: true });

/**
 * Mounted lifecycle hook after initial render.
 * Renders QR code.
 */
onMounted(async (): Promise<void> => {
    setViewState();

    intervalID.value = setInterval(async () => {
        try {
            await billingStore.getPaymentsWithConfirmations();
        } catch { /* empty */ }
    }, 20000); // get payments every 20 seconds.

    if (!canvas.value) {
        return;
    }

    try {
        await QRCode.toCanvas(canvas.value, wallet.value.address, { width: 124 });
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
    }
});

onBeforeUnmount(() => {
    clearInterval(intervalID.value);

    if (viewState.value === ViewState.Success) {
        billingStore.clearPendingPayments();
    }
});
</script>