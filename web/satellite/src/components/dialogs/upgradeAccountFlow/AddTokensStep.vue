// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <template v-if="wallet.address">
        <p class="text-body-2">
            <template v-if="isPaidTier">
                Send STORJ Tokens to the deposit address to credit your Storj account and receive a 10% bonus on your deposit.
            </template>
            <template v-else>
                Send at least $10 in STORJ Tokens to the following deposit address to upgrade to a Pro account.
                Your account will be upgraded after your transaction receives {{ neededConfirmations }} confirmations.
                If your account is not automatically upgraded, please fill out this
                <a
                    :href="supportUrl"
                    target="_blank"
                    class="link"
                    rel="noopener noreferrer"
                >limit increase request form</a>.
            </template>
            <br>
        </p>

        <template v-if="!isAcknowledged">
            <v-alert
                class="mt-3 mb-2"
                density="compact"
                variant="tonal"
                type="warning"
            >
                <template #prepend>
                    <v-icon :icon="Info" />
                </template>
                <template #text>
                    <p class="font-weight-bold">
                        The deposit address only supports ERC20 STORJ tokens sent via:
                    </p>
                    <p>Ethereum (L1) network </p>
                    <p>zkSync Era (L2) network</p>
                    <p>
                        <span class="font-weight-bold">Warning:</span> Sending any other token
                        type or using any other network will result in permanent loss of funds.
                    </p>
                </template>
            </v-alert>

            <v-alert
                class="mt-3 mb-2"
                density="compact"
                variant="tonal"
                @click="boxChecked = !boxChecked"
            >
                <template #prepend>
                    <v-checkbox v-model="boxChecked" density="compact" />
                </template>
                <template #text>
                    <p class="cursor-default font-weight-bold">
                        Safety Confirmation
                    </p>
                    <p class="cursor-default">
                        I confirm that I will only send ERC20 STORJ tokens via
                        the Ethereum or zkSync Era networks. I understand that
                        using any other token or network will result in permanent
                        loss of funds with no possibility of recovery.
                    </p>
                </template>
            </v-alert>
        </template>
        <template v-else>
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

            <v-alert
                v-if="!isRoot && !isPaidTier"
                class="mt-3 mb-2"
                density="compact"
                variant="tonal"
                type="info"
                text="You can continue using the app, and your upgrade will be applied automatically once your STORJ tokens are received."
            />
        </template>

        <v-card-actions class="px-0">
            <v-row class="ma-0 gap">
                <template v-if="!isAcknowledged">
                    <v-col class="px-0">
                        <v-btn
                            block
                            variant="outlined"
                            color="default"
                            @click="() => isRoot ? emit('close') : emit('back')"
                        >
                            {{ isRoot ? 'Cancel' : 'Back' }}
                        </v-btn>
                    </v-col>
                    <v-col class="px-0">
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :disabled="!boxChecked"
                            @click="isAcknowledged = true"
                        >
                            Show Deposit Address
                        </v-btn>
                    </v-col>
                </template>
                <template v-else>
                    <v-col v-if="!isRoot && viewState !== ViewState.Success" class="px-0">
                        <v-btn
                            block
                            variant="outlined"
                            color="default"
                            @click="() => emit('back')"
                        >
                            Back
                        </v-btn>
                    </v-col>
                    <v-col class="px-0">
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            @click="() => isRoot ? emit('close') : emit('success')"
                        >
                            {{ isRoot ? 'Done' : 'Next' }}
                        </v-btn>
                    </v-col>
                </template>
            </v-row>
        </v-card-actions>
    </template>
    <template v-else>
        <v-alert type="error" variant="tonal">
            <template #prepend>
                <v-icon :icon="Info" />
            </template>
            <template #text>
                <p class="font-weight-bold">
                    You need to generate a deposit address before you can add STORJ tokens.
                </p>
            </template>
        </v-alert>
    </template>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import QRCode from 'qrcode';
import { VTooltip, VBtn, VIcon, VCol, VRow, VCardActions, VAlert, VCheckbox } from 'vuetify/components';
import { Copy, Info } from 'lucide-vue-next';

import { useBillingStore } from '@/store/modules/billingStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/composables/useNotify';
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
const isAcknowledged = ref(false);
const boxChecked = ref(false);

defineProps<{
    // whether this step is the first step in a flow
    isRoot?: boolean;
}>();

const emit = defineEmits<{
    back: [];
    success: [];
    close: [];
}>();

const supportUrl = computed<string>(() => `${configStore.supportUrl}?ticket_form_id=360000683212`);

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
const isPaidTier = computed((): boolean => usersStore.state.user.isPaid);

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

watch(canvas, async newVal => {
    if (!newVal) return;
    // canvas only available after isAcknowledged is true
    billingStore.startPaymentsPolling();
    try {
        await QRCode.toCanvas(canvas.value, wallet.value.address, { width: 124 });
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
    }
});

/**
 * Mounted lifecycle hook after initial render.
 * Renders QR code.
 */
onMounted(async (): Promise<void> => {
    setViewState();
});
</script>

<style scoped lang="scss">
.gap {
    column-gap: 12px;
}
</style>
