// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row>
            <v-col>
                <PageTitleComponent title="Account Billing" />
            </v-col>
        </v-row>

        <v-card color="default" class="mt-2 mb-6" rounded="md">
            <v-tabs
                v-model="tab"
                color="primary"
                center-active
                show-arrows
                grow
            >
                <v-tab :value="TABS.overview">Overview</v-tab>
                <v-tab :value="TABS['payment-methods']">Payment Methods</v-tab>
                <v-tab :value="TABS['billing-history']">Billing History</v-tab>
                <v-tab v-if="tokenPaymentsShown" :value="TABS.transactions">STORJ Transactions</v-tab>
                <v-tab v-if="billingInformationUIEnabled" :value="TABS['billing-information']">Billing Information</v-tab>
            </v-tabs>
        </v-card>

        <v-window v-model="tab">
            <v-window-item :value="TABS.overview" class="pb-2">
                <overview-tab
                    @to-billing-history-tab="tab = TABS['billing-history']"
                    @add-tokens-clicked="onAddTokensClicked"
                />
            </v-window-item>

            <v-window-item :value="TABS['payment-methods']" class="pb-2">
                <v-row>
                    <v-col v-if="tokenPaymentsShown" cols="12" sm="12" md="6" lg="6" xl="6" xxl="4">
                        <StorjTokenCardComponent ref="tokenCardComponent" @history-clicked="tab = TABS.transactions" />
                    </v-col>

                    <CreditCards />
                </v-row>
            </v-window-item>

            <v-window-item :value="TABS['billing-history']" class="pb-2">
                <billing-history-tab />
            </v-window-item>

            <v-window-item v-if="tokenPaymentsShown" :value="TABS.transactions" class="pb-2">
                <token-transactions-table-component />
            </v-window-item>

            <v-window-item v-if="billingInformationUIEnabled" :value="TABS['billing-information']" class="pb-2">
                <billing-information-tab />
            </v-window-item>
        </v-window>
    </v-container>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, onMounted, ref } from 'vue';
import {
    VCard,
    VCol,
    VContainer,
    VRow,
    VTab,
    VTabs,
    VWindow,
    VWindowItem,
} from 'vuetify/components';
import { useRoute, useRouter } from 'vue-router';

import { ROUTES } from '@/router';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useBillingStore } from '@/store/modules/billingStore';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import BillingHistoryTab from '@/components/billing/BillingHistoryTab.vue';
import StorjTokenCardComponent from '@/components/billing/StorjTokenCardComponent.vue';
import TokenTransactionsTableComponent from '@/components/billing/TokenTransactionsTableComponent.vue';
import BillingInformationTab from '@/components/billing/BillingInformationTab.vue';
import OverviewTab from '@/components/billing/OverviewTab.vue';
import CreditCards from '@/components/billing/CreditCards.vue';

enum TABS {
    overview = 'overview',
    'payment-methods' = 'payment-methods',
    'billing-history' = 'billing-history',
    transactions = 'transactions',
    'billing-information' = 'billing-information',
}

interface IStorjTokenCardComponent {
    onAddTokens(): Promise<void>;
}

const configStore = useConfigStore();
const usersStore = useUsersStore();
const appStore = useAppStore();
const billingStore = useBillingStore();

const router = useRouter();
const route = useRoute();

const tokenCardComponent = ref<IStorjTokenCardComponent>();

const billingInformationUIEnabled = computed<boolean>(() => configStore.state.config.billingInformationTabEnabled);
const tokenDepositsEnabled = computed<boolean>(() => configStore.tokenDepositsEnabled);
/**
 * Whether to show the token balance and transactions. They stay visible for users
 * with a wallet even when deposits are disabled, so earlier deposits can be reviewed.
 */
const tokenPaymentsShown = computed<boolean>(() => tokenDepositsEnabled.value || !!billingStore.state.wallet.address);
const userPaidTier = computed<boolean>(() => usersStore.state.user.isPaid);
const isMemberAccount = computed<boolean>(() => usersStore.state.user.isMember);

/**
 * Returns the tabs that are currently shown.
 */
const shownTabs = computed<TABS[]>(() => [
    TABS.overview,
    TABS['payment-methods'],
    TABS['billing-history'],
    ...(tokenPaymentsShown.value ? [TABS.transactions] : []),
    ...(billingInformationUIEnabled.value ? [TABS['billing-information']] : []),
]);

/**
 * Returns the last billing tab the user was on, to be used as the current.
 * Falls back to the overview if that tab is unknown or not shown.
 */
const tab = computed<TABS>({
    get: () => {
        const value = route.query.tab as TABS;
        return shownTabs.value.includes(value) ? value : TABS.overview;
    },
    set: (value: TABS) => {
        router.push({ query: { tab: value } });
    },
});

function onAddTokensClicked(): void {
    if (!tokenDepositsEnabled.value) return;

    if (!userPaidTier.value) {
        appStore.toggleUpgradeFlow(true);
        return;
    }

    tab.value = TABS['payment-methods'];
    tokenCardComponent.value?.onAddTokens();
}

onBeforeMount(() => {
    if (!configStore.getBillingEnabled(usersStore.state.user) || isMemberAccount.value) {
        router.replace({ name: ROUTES.AccountSettings.name });
    }
});

onMounted(() => {
    // When deposits are enabled, the token card fetches the wallet itself.
    if (!tokenDepositsEnabled.value) billingStore.getWallet().catch(_ => {});
});
</script>
