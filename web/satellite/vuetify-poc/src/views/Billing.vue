// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <!-- <v-breadcrumbs :items="['My Account', 'Billing']" class="pl-0"></v-breadcrumbs> -->

        <h1 class="text-h5 font-weight-bold mb-6">Billing</h1>

        <v-card variant="flat" :border="true" color="default" class="mb-6 rounded">
            <v-tabs
                v-model="tab"
                color="default"
                center-active
                show-arrows
                grow
            >
                <v-tab>
                    Overview
                </v-tab>
                <v-tab>
                    Payment Methods
                </v-tab>
                <v-tab>
                    STORJ Transactions
                </v-tab>
                <v-tab>
                    Billing History
                </v-tab>
                <v-tab>
                    Billing Information
                </v-tab>
            </v-tabs>
        </v-card>

        <v-window v-model="tab">
            <v-window-item>
                <v-row>
                    <v-col cols="12" sm="4">
                        <v-card title="Total Cost" subtitle="Estimated for June 2023" variant="flat" :border="true" rounded="xlg">
                            <v-card-text>
                                <v-chip rounded color="success" variant="outlined" class="font-weight-bold mb-2">$24</v-chip>
                                <v-divider class="my-4" />
                                <v-btn variant="outlined" color="default" size="small" class="mr-2">View Billing History</v-btn>
                                <!-- <v-btn variant="tonal" color="default" size="small" class="mr-2">Payment Methods</v-btn> -->
                            </v-card-text>
                        </v-card>
                    </v-col>

                    <v-col cols="12" sm="4">
                        <v-card title="STORJ Token Balance" subtitle="Your STORJ Token Wallet" variant="flat" :border="true" rounded="xlg">
                            <v-card-text>
                                <v-chip rounded color="success" variant="outlined" class="font-weight-bold mb-2">$5,284</v-chip>
                                <v-divider class="my-4" />
                                <v-btn variant="outlined" color="default" size="small" class="mr-2">+ Add STORJ Tokens</v-btn>
                                <!-- <v-btn variant="tonal" color="default" size="small" class="mr-2">View Transactions</v-btn> -->
                            </v-card-text>
                        </v-card>
                    </v-col>

                    <v-col cols="12" sm="4">
                        <v-card title="Coupon / Free Usage" subtitle="Active / No Expiration" variant="flat" :border="true" rounded="xlg">
                            <v-card-text>
                                <v-chip rounded color="success" variant="outlined" class="font-weight-bold mb-2">$1.65 off</v-chip>
                                <v-divider class="my-4" />
                                <v-btn variant="outlined" color="default" size="small" class="mr-2">+ Add Coupon</v-btn>
                            </v-card-text>
                        </v-card>
                    </v-col>
                </v-row>

                <v-row>
                    <v-col>
                        <h4 class="mt-4">Costs per project</h4>
                    </v-col>
                </v-row>

                <v-row>
                    <v-col>
                        <v-card rounded="lg" variant="flat" :border="true" class="mb-4">
                            <v-expansion-panels>
                                <v-expansion-panel
                                    title="My First Project"
                                    text="Costs..."
                                    rounded="lg"
                                />
                            </v-expansion-panels>
                        </v-card>
                        <v-card rounded="lg" variant="flat" :border="true" class="mb-4">
                            <v-expansion-panels>
                                <v-expansion-panel
                                    title="Storj Labs"
                                    text="Costs..."
                                    rounded="lg"
                                />
                            </v-expansion-panels>
                        </v-card>
                        <v-card rounded="lg" variant="flat" :border="true" class="mb-4">
                            <v-expansion-panels>
                                <v-expansion-panel
                                    title="Pictures"
                                    text="Costs..."
                                    rounded="lg"
                                />
                            </v-expansion-panels>
                        </v-card>
                    </v-col>
                </v-row>
            </v-window-item>

            <v-window-item>
                <v-row>
                    <v-col cols="12" sm="4">
                        <v-card title="STORJ Token" variant="flat" :border="true" rounded="xlg">
                            <v-card-text>
                                <v-chip rounded color="default" variant="tonal" class="font-weight-bold mr-2">STORJ</v-chip>
                                <!-- <v-chip rounded color="success" variant="tonal" class="font-weight-bold mr-2">Primary</v-chip> -->
                                <v-divider class="my-4" />
                                <p>Deposit Address</p>
                                <v-chip rounded color="default" variant="text" class="font-weight-bold mt-2 pl-0">0x0683 . . . 2759</v-chip>
                                <v-divider class="my-4" />
                                <p>Total Balance</p>
                                <v-chip rounded color="success" variant="outlined" class="font-weight-bold mt-2">$5,284</v-chip>
                                <v-divider class="my-4" />
                                <v-btn variant="flat" color="success" size="small" class="mr-2">+ Add STORJ Tokens</v-btn>
                                <v-btn variant="outlined" color="default" size="small" class="mr-2">View Transactions</v-btn>
                            </v-card-text>
                        </v-card>
                    </v-col>

                    <v-col v-for="(card, i) in creditCards" :key="i" cols="12" sm="4">
                        <CreditCardComponent :card="card" />
                    </v-col>

                    <v-col cols="12" sm="4">
                        <AddCreditCardComponent />
                    </v-col>
                </v-row>
            </v-window-item>

            <v-window-item>
                <v-card variant="flat" :border="true" rounded="xlg">
                    <v-text-field
                        v-model="search"
                        label="Search"
                        prepend-inner-icon="mdi-magnify"
                        single-line
                        hide-details
                    />

                    <v-data-table
                        v-model="selected"
                        :sort-by="sortBy"
                        :headers="headers"
                        :items="invoices"
                        :search="search"
                        class="elevation-1"
                        show-select
                        hover
                    >
                        <template #item.date="{ item }">
                            <span class="font-weight-bold">
                                {{ item.raw.date }}
                            </span>
                        </template>
                        <template #item.status="{ item }">
                            <v-chip :color="getColor(item.raw.status)" variant="tonal" size="small" rounded="xl" class="font-weight-bold">
                                {{ item.raw.status }}
                            </v-chip>
                        </template>
                    </v-data-table>
                </v-card>
            </v-window-item>

            <v-window-item>
                <v-card variant="flat" :border="true" rounded="xlg">
                    <v-text-field
                        v-model="search"
                        label="Search"
                        prepend-inner-icon="mdi-magnify"
                        single-line
                        hide-details
                    />

                    <v-data-table
                        v-model="selected"
                        :sort-by="sortBy"
                        :headers="headers"
                        :items="invoices"
                        :search="search"
                        class="elevation-1"
                        show-select
                        hover
                    >
                        <template #item.date="{ item }">
                            <span class="font-weight-bold">
                                {{ item.raw.date }}
                            </span>
                        </template>
                        <template #item.status="{ item }">
                            <v-chip :color="getColor(item.raw.status)" variant="tonal" size="small" rounded="xl" class="font-weight-bold">
                                {{ item.raw.status }}
                            </v-chip>
                        </template>
                    </v-data-table>
                </v-card>
            </v-window-item>

            <v-window-item>
                <v-row>
                    <v-col cols="12" sm="4">
                        <v-card title="Billing Information" subtitle="Add info for your invoices." variant="flat" :border="true" rounded="xlg">
                            <v-card-text>
                                <!-- <v-chip rounded color="purple2" variant="tonal" class="font-weight-bold mb-2">$0</v-chip> -->
                                <p>You can add personal or company info, billing email, and VAT.</p>
                                <v-divider class="my-4" />
                                <v-btn color="primary" size="small">+ Add Billing Information</v-btn>
                            </v-card-text>
                        </v-card>
                    </v-col>
                </v-row>
            </v-window-item>
        </v-window>
    </v-container>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import {
    VContainer,
    VCard,
    VTabs,
    VTab,
    VWindow,
    VWindowItem,
    VRow,
    VCol,
    VCardText,
    VChip,
    VDivider,
    VBtn,
    VExpansionPanels,
    VExpansionPanel,
    VTextField,
} from 'vuetify/components';
import { VDataTable } from 'vuetify/labs/components';

import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import { CreditCard } from '@/types/payments';

import CreditCardComponent from '@poc/components/CreditCardComponent.vue';
import AddCreditCardComponent from '@poc/components/AddCreditCardComponent.vue';

const tab = ref<string>('Overview');
const search = ref<string>('');
const selected = ref([]);

const billingStore = useBillingStore();

const { isLoading, withLoading } = useLoading();

const creditCards = computed((): CreditCard[] => {
    return billingStore.state.creditCards;
});

const sortBy = [{ key: 'date', order: 'asc' }];
const headers = [
    { title: 'Date', key: 'date' },
    { title: 'Amount', key: 'amount' },
    { title: 'Status', key: 'status' },
    { title: 'Invoice', key: 'invoice' },
];
const invoices = [
    {
        date: 'Jun 2023',
        status: 'Pending',
        amount: '$23',
        invoice: 'Invoice',
    },
    {
        date: 'May 2023',
        status: 'Unpaid',
        amount: '$821',
        invoice: 'Invoice',
    },
    {
        date: 'Apr 2023',
        status: 'Paid',
        amount: '$9,345',
        invoice: 'Invoice',
    },
];

function getColor(status: string): string {
    if (status === 'Paid') return 'success';
    if (status === 'Pending') return 'warning';
    return 'error';
}

onMounted(() => {
    withLoading(async () => {
        await billingStore.getCreditCards();
    });
});
</script>