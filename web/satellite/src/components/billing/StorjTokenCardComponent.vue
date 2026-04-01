// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card title="STORJ Token" class="pa-2">
        <v-card-text>
            <v-row class="ma-0 align-center">
                <v-chip color="primary" size="small" variant="tonal" class="font-weight-bold mr-2">STORJ</v-chip>
                <v-chip color="default" size="small" variant="tonal" class="font-weight-bold">
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
                            If the STORJ token balance runs out, the default card will be charged.
                            <a class="link" href="https://docs.storj.io/support/account-management-billing/payment-methods" target="_blank" rel="noopener noreferrer">
                                Learn more
                            </a>
                        </v-tooltip>
                    </span>
                </v-chip>
            </v-row>
            <v-divider class="my-6 border-0" />
            <p>Deposit Address</p>
            <v-row class="ma-0 mt-2 align-center">
                <v-chip color="default" variant="text" class="font-weight-bold px-0 mr-4" @click="isAddTokenDialogOpen = true">
                    {{ shortAddress || '-' }}
                </v-chip>
                <v-tooltip v-if="wallet.address" v-model="isCopyTooltip" location="start">
                    <template #activator="{ props: activatorProps }">
                        <v-btn
                            v-bind="activatorProps"
                            :icon="Copy"
                            variant="text"
                            density="compact"
                            aria-roledescription="copy-btn"
                            color="primary"
                            @click="isAddTokenDialogOpen = true"
                        />
                    </template>
                    Copy
                </v-tooltip>
            </v-row>
            <v-divider class="my-6 border-0" />
            <p>Total Balance</p>
            <v-chip variant="text" class="text-primary pl-0 font-weight-bold pt-2">{{ balance || '-' }}</v-chip>
            <v-divider class="my-6 border-0" />
            <v-btn v-if="wallet.address" variant="flat" color="primary" :loading="isLoading" class="mt-2 mr-2" :prepend-icon="Plus" @click="onAddTokens">Add STORJ Tokens</v-btn>
            <v-btn v-else variant="flat" color="primary" :loading="isLoading" class="mt-2" :prepend-icon="Plus" @click="claimWalletClick">Generate Deposit Address</v-btn>
            <v-btn v-if="wallet.address" variant="outlined" color="default" :loading="isLoading" class="mt-2" @click="emit('historyClicked')">View Transactions</v-btn>
        </v-card-text>
    </v-card>

    <AddTokensDialog v-model="isAddTokenDialogOpen" />
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VChip, VDivider, VTooltip, VRow, VIcon } from 'vuetify/components';
import { computed, onMounted, ref } from 'vue';
import { Info, Plus, Copy } from 'lucide-vue-next';

import { Wallet } from '@/types/payments';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useAppStore } from '@/store/modules/appStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useUsersStore } from '@/store/modules/usersStore';

import AddTokensDialog from '@/components/dialogs/AddTokensDialog.vue';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
const billingStore = useBillingStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const isAddTokenDialogOpen = ref<boolean>(false);
const isCopyTooltip = ref<boolean>(false);

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
    return wallet.value.balance.formattedValue;
});

/**
 * Returns wallet from store.
 */
const wallet = computed((): Wallet => {
    return billingStore.state.wallet as Wallet;
});

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
    if (!usersStore.state.user.isPaid) {
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

    if (!usersStore.state.user.isPaid) {
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
