// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card title="STORJ Token" variant="flat" :border="true" rounded="xlg">
        <v-card-text>
            <v-chip rounded color="default" variant="tonal" class="font-weight-bold mr-2">STORJ</v-chip>
            <v-divider class="my-4" />
            <p>Deposit Address</p>
            <v-chip rounded color="default" variant="text" class="font-weight-bold mt-2 px-0" @click="copyAddress">
                {{ shortAddress || '-------' }}
                <v-tooltip
                    v-if="wallet.address"
                    activator="parent"
                    location="top"
                >
                    {{ wallet.address }}
                </v-tooltip>
            </v-chip>
            <v-divider class="my-4" />
            <p>Total Balance</p>
            <v-chip rounded color="success" variant="outlined" class="font-weight-bold mt-2">{{ balance || '------' }}</v-chip>
            <v-divider class="my-4" />
            <v-btn v-if="wallet.address" variant="flat" color="success" size="small" :loading="isLoading" class="mr-2">+ Add STORJ Tokens</v-btn>
            <v-btn v-else variant="flat" color="success" size="small" :loading="isLoading" @click="claimWalletClick">Create New Wallet</v-btn>
            <v-btn v-if="wallet.address" variant="outlined" color="default" size="small" :loading="isLoading" @click="emit('historyClicked')">View Transactions</v-btn>
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VChip, VDivider, VTooltip } from 'vuetify/components';
import { computed, onMounted } from 'vue';

import { Wallet } from '@/types/payments';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';

const billingStore = useBillingStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

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
    withLoading(async () => {
        try {
            await claimWallet();
            notify.success('Wallet created successfully.');
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.BILLING_STORJ_TOKEN_CONTAINER);
        }
    });
}

onMounted(() => {
    getWallet();
});
</script>