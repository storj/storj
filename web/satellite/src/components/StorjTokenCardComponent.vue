// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card title="STORJ Token" variant="flat">
        <v-card-text>
            <v-row class="ma-0 align-center">
                <v-chip color="default" size="small" variant="tonal" class="font-weight-bold mr-2">STORJ</v-chip>
                <v-chip color="info" size="small" variant="tonal" class="font-weight-bold">
                    Default
                    <span class="d-inline-flex ml-1">
                        <v-icon class="text-cursor-pointer" :icon="Info" />
                        <v-tooltip
                            class="text-center"
                            activator="parent"
                            location="top"
                            max-width="300px"
                            open-delay="150"
                            close-delay="150"
                        >
                            If the STORJ token balance runs out, the default credit card will be charged.
                            <a class="link" href="https://docs.storj.io/support/account-management-billing/payment-methods" target="_blank" rel="noopener noreferrer">
                                Learn more
                            </a>
                        </v-tooltip>
                    </span>
                </v-chip>
            </v-row>
            <v-divider class="my-4" />
            <p>Deposit Address</p>
            <v-row class="ma-0 mt-2 align-center">
                <v-chip color="default" variant="text" class="font-weight-bold px-0 mr-4" @click="copyAddress">
                    {{ shortAddress || '-------' }}
                    <v-tooltip
                        v-if="wallet.address"
                        activator="parent"
                        location="top"
                    >
                        {{ wallet.address }}
                    </v-tooltip>
                </v-chip>
                <input-copy-button v-if="wallet.address" :value="wallet.address" />
            </v-row>
            <v-divider class="my-4" />
            <p>Total Balance</p>
            <v-chip color="success" variant="tonal" class="font-weight-bold mt-2">{{ balance || '------' }}</v-chip>
            <v-divider class="mt-4 mb-2" />
            <v-btn v-if="wallet.address" variant="flat" color="success" size="small" rounded="md" :loading="isLoading" class="mt-2 mr-2" @click="onAddTokens">+ Add STORJ Tokens</v-btn>
            <v-btn v-else variant="flat" color="success" size="small" rounded="md" :loading="isLoading" class="mt-2" @click="claimWalletClick">Create New Wallet</v-btn>
            <v-btn v-if="wallet.address" variant="tonal" color="default" size="small" rounded="md" :loading="isLoading" class="mt-2" @click="emit('historyClicked')">View Transactions</v-btn>
        </v-card-text>
    </v-card>

    <AddTokensDialog v-model="isAddTokenDialogOpen" />
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VChip, VDivider, VTooltip, VRow, VIcon } from 'vuetify/components';
import { computed, onMounted, ref } from 'vue';
import { Info } from 'lucide-vue-next';

import { Wallet } from '@/types/payments';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useUsersStore } from '@/store/modules/usersStore';

import AddTokensDialog from '@/components/dialogs/AddTokensDialog.vue';
import InputCopyButton from '@/components/InputCopyButton.vue';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
const billingStore = useBillingStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const isAddTokenDialogOpen = ref<boolean>(false);

const emit = defineEmits(['historyClicked']);

/**
 * Returns shortened wallet address.
 */
const shortAddress = computed((): string => {
    if (!wallet.value.address) {
        return '';
    }
    const addr = wallet.value.address;
    return `${addr.substring(0, 6)} . . . ${addr.substring(addr.length - 4, addr.length)}`;
});

/**
 * Returns a formatted wallet balance from store.
 */
const balance = computed((): string => {
    if (!wallet.value.address) {
        return '';
    }
    return '$' + wallet.value.balance.value.toLocaleString();
});

/**
 * Returns wallet from store.
 */
const wallet = computed((): Wallet => {
    return billingStore.state.wallet as Wallet;
});

/**
 * Copies the wallet address.
 */
function copyAddress(): void {
    if (!wallet.value.address) {
        return;
    }
    navigator.clipboard.writeText(wallet.value.address);
}

/**
 * getWallet tries to get an existing wallet for this user. this will not claim a wallet.
 */
function getWallet(): void {
    if (wallet.value.address) {
        return;
    }

    withLoading(async () => {
        await billingStore.getWallet().catch(_ => {});
    });
}

/**
 * claimWallet claims a wallet for the current account.
 */
async function claimWallet(): Promise<void> {
    if (wallet.value.address) {
        return;
    }
    await billingStore.claimWallet();
}

/**
 * Called when "Create New Wallet" button is clicked.
 */
function claimWalletClick(): void {
    if (!usersStore.state.user.paidTier) {
        appStore.toggleUpgradeFlow(true);
        return;
    }

    withLoading(async () => {
        try {
            await claimWallet();
            notify.success('Wallet created successfully.');
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.BILLING_STORJ_TOKEN_CONTAINER);
        }
    });
}

/**
 * Open the add tokens step of the upgrade modal
 * Conditionally claim a wallet before that.
 */
function onAddTokens(): void {
    analyticsStore.eventTriggered(AnalyticsEvent.ADD_FUNDS_CLICKED);

    if (!usersStore.state.user.paidTier) {
        appStore.toggleUpgradeFlow(true);
        return;
    }

    withLoading(async () => {
        if (!wallet.value.address) {
            // not possible from this component
            // but this function is exported and used Billing.vue
            try {
                await billingStore.claimWallet();
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.BILLING_STORJ_TOKEN_CONTAINER);
                return;
            }
        }

        isAddTokenDialogOpen.value = true;
    });
}

defineExpose({ onAddTokens });

onMounted(() => {
    getWallet();
});
</script>
