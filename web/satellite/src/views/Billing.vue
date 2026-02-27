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
                <v-tab>Overview</v-tab>
                <v-tab>Payment Methods</v-tab>
                <v-tab>STORJ Transactions</v-tab>
                <v-tab>Billing History</v-tab>
                <v-tab v-if="billingInformationUIEnabled">Billing Information</v-tab>
            </v-tabs>
        </v-card>

        <v-window v-model="tab">
            <v-window-item class="pb-2">
                <overview-tab
                    @to-billing-history-tab="tab = TABS['billing-history']"
                    @add-tokens-clicked="onAddTokensClicked"
                />
            </v-window-item>

            <v-window-item class="pb-2">
                <v-row>
                    <v-col cols="12" sm="12" md="6" lg="6" xl="4">
                        <StorjTokenCardComponent ref="tokenCardComponent" @history-clicked="tab = TABS.transactions" />
                    </v-col>

                    <CreditCards />
                </v-row>
            </v-window-item>

            <v-window-item class="pb-2">
                <token-transactions-table-component />
            </v-window-item>

            <v-window-item class="pb-2">
                <billing-history-tab />
            </v-window-item>

            <v-window-item v-if="billingInformationUIEnabled" class="pb-2">
                <billing-information-tab />
            </v-window-item>
        </v-window>
    </v-container>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';
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

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import BillingHistoryTab from '@/components/billing/BillingHistoryTab.vue';
import StorjTokenCardComponent from '@/components/billing/StorjTokenCardComponent.vue';
import TokenTransactionsTableComponent from '@/components/billing/TokenTransactionsTableComponent.vue';
import BillingInformationTab from '@/components/billing/BillingInformationTab.vue';
import OverviewTab from '@/components/billing/OverviewTab.vue';
import CreditCards from '@/components/billing/CreditCards.vue';

enum TABS {
    overview,
    'payment-methods',
    transactions,
    'billing-history',
    'billing-information',
}

interface IStorjTokenCardComponent {
    onAddTokens(): Promise<void>;
}

const configStore = useConfigStore();
const usersStore = useUsersStore();
const appStore = useAppStore();

const router = useRouter();
const route = useRoute();

const tokenCardComponent = ref<IStorjTokenCardComponent>();

const billingInformationUIEnabled = computed<boolean>(() => configStore.state.config.billingInformationTabEnabled);
const userPaidTier = computed<boolean>(() => usersStore.state.user.isPaid);
const isMemberAccount = computed<boolean>(() => usersStore.state.user.isMember);

/**
 * Returns the last billing tab the user was on,
 * to be used as the current.
 */
const tab = computed({
    get: () => {
        const tabStr = route.query['tab'] as keyof typeof TABS;
        return TABS[tabStr] ?? 0;
    },
    set: (value: number) => {
        router.push({ query: { tab: TABS[value] ?? TABS[tab.value] } });
    },
});

function onAddTokensClicked(): void {
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
</script>
