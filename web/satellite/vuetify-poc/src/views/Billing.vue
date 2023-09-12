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
                        <v-card
                            title="Total Cost"
                            :subtitle="`Estimated for ${new Date().toLocaleString('en-US', { month: 'long', year: 'numeric' })}`"
                            variant="flat"
                            :border="true"
                            rounded="xlg"
                        >
                            <v-card-text>
                                <div v-if="isLoading" class="pb-2 text-center">
                                    <v-progress-circular class="ma-0" color="primary" size="30" indeterminate />
                                </div>
                                <v-chip v-else rounded color="success" variant="outlined" class="font-weight-bold mb-2">
                                    {{ centsToDollars(priceSummary) }}
                                </v-chip>
                                <v-divider class="my-4" />
                                <v-btn variant="outlined" color="default" size="small" class="mr-2">View Billing History</v-btn>
                                <!-- <v-btn variant="tonal" color="default" size="small" class="mr-2">Payment Methods</v-btn> -->
                            </v-card-text>
                        </v-card>
                    </v-col>

                    <v-col cols="12" sm="4">
                        <v-card title="STORJ Token Balance" subtitle="Your STORJ Token Wallet" variant="flat" :border="true" rounded="xlg">
                            <v-card-text>
                                <div v-if="isLoading" class="pb-2 text-center">
                                    <v-progress-circular class="ma-0" color="primary" size="30" indeterminate />
                                </div>
                                <v-chip v-else rounded color="success" variant="outlined" class="font-weight-bold mb-2">
                                    {{ formattedTokenBalance }}
                                </v-chip>
                                <v-divider class="my-4" />
                                <v-btn variant="outlined" color="default" size="small" class="mr-2">+ Add STORJ Tokens</v-btn>
                                <!-- <v-btn variant="tonal" color="default" size="small" class="mr-2">View Transactions</v-btn> -->
                            </v-card-text>
                        </v-card>
                    </v-col>

                    <v-col cols="12" sm="4">
                        <v-card
                            v-if="isLoading"
                            class="d-flex align-center justify-center"
                            height="200"
                            rounded="xlg"
                            border
                        >
                            <v-progress-circular color="primary" size="48" indeterminate />
                        </v-card>
                        <v-card
                            v-else-if="coupon"
                            :title="`Coupon / ${coupon.name}`"
                            :subtitle="`${isCouponActive ? 'Active' : 'Expired'} / ${couponExpiration}`"
                            rounded="xlg"
                            border
                        >
                            <v-card-text>
                                <v-chip
                                    :color="isCouponActive ? 'success' : 'error'"
                                    variant="outlined"
                                    class="font-weight-bold mb-2"
                                    rounded
                                >
                                    {{ couponDiscount }}
                                </v-chip>
                                <v-divider class="my-4" />
                                <v-btn variant="outlined" color="default" size="small" class="mr-2">+ Add Coupon</v-btn>
                            </v-card-text>
                        </v-card>
                        <v-card
                            v-else
                            class="billing__new-coupon-card d-flex align-center justify-center"
                            color="primary"
                            variant="text"
                            height="200"
                            link
                            border
                        >
                            <v-icon icon="mdi-plus" class="mr-1" size="small" />
                            <span class="text-decoration-underline mr-1">Apply New Coupon</span>
                        </v-card>
                    </v-col>
                </v-row>

                <v-row v-if="isRollupLoading" justify="center" align="center">
                    <v-col cols="auto">
                        <v-progress-circular indeterminate />
                    </v-col>
                </v-row>
                <usage-and-charges-component v-else :project-ids="projectIDs" />
            </v-window-item>

            <v-window-item>
                <v-row>
                    <v-col cols="12" sm="4">
                        <StorjTokenCardComponent @historyClicked="goToTransactionsTab" />
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
                <token-transactions-table-component />
            </v-window-item>

            <v-window-item>
                <billing-history-tab />
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
    VProgressCircular,
    VIcon,
} from 'vuetify/components';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { Coupon, CouponDuration, CreditCard } from '@/types/payments';
import { centsToDollars } from '@/utils/strings';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { useProjectsStore } from '@/store/modules/projectsStore';

import CreditCardComponent from '@poc/components/CreditCardComponent.vue';
import AddCreditCardComponent from '@poc/components/AddCreditCardComponent.vue';
import BillingHistoryTab from '@poc/components/billing/BillingHistoryTab.vue';
import UsageAndChargesComponent from '@poc/components/billing/UsageAndChargesComponent.vue';
import StorjTokenCardComponent from '@poc/components/StorjTokenCardComponent.vue';
import TokenTransactionsTableComponent from '@poc/components/TokenTransactionsTableComponent.vue';

const tab = ref(0);
const search = ref<string>('');
const selected = ref([]);

const billingStore = useBillingStore();
const projectsStore = useProjectsStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const isRollupLoading = ref(true);

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

/**
 * projectIDs is an array of all of the project IDs for which there exist project usage charges.
 */
const projectIDs = computed((): string[] => {
    return projectsStore.state.projects
        .filter(proj => billingStore.state.projectCharges.hasProject(proj.id))
        .sort((proj1, proj2) => proj1.name.localeCompare(proj2.name))
        .map(proj => proj.id);
});

/**
 * Returns price summary of all project usages.
 */
const priceSummary = computed((): number => {
    return billingStore.state.projectCharges.getPrice();
});

/**
 * Returns STORJ token balance from store.
 */
const formattedTokenBalance = computed((): string => {
    return billingStore.state.balance.formattedCoins;
});

/**
 * Returns the coupon applied to the user's account.
 */
const coupon = computed((): Coupon | null => {
    return billingStore.state.coupon;
});

/**
 * Returns the expiration date of the coupon.
 */
const couponExpiration = computed((): string => {
    const c = coupon.value;
    if (!c) return '';

    const exp = c.expiresAt;
    if (!exp || c.duration === CouponDuration.Forever) {
        return 'No Expiration';
    }
    return `Expires on ${exp.getDate()} ${SHORT_MONTHS_NAMES[exp.getMonth()]} ${exp.getFullYear()}`;
});

/**
 * Returns the coupon's discount amount.
 */
const couponDiscount = computed((): string => {
    const c = coupon.value;
    if (!c) return '';

    if (c.percentOff !== 0) {
        return `${parseFloat(c.percentOff.toFixed(2)).toString()}% off`;
    }
    return `$${(c.amountOff / 100).toFixed(2).replace('.00', '')} off`;
});

/**
 * Returns the whether the coupon is active.
 */
const isCouponActive = computed((): boolean => {
    const now = Date.now();
    const c = coupon.value;
    return !!c && (c.duration === 'forever' || (!!c.expiresAt && now < c.expiresAt.getTime()));
});

function goToTransactionsTab() {
    tab.value = 2;
}

function getColor(status: string): string {
    if (status === 'Paid') return 'success';
    if (status === 'Pending') return 'warning';
    return 'error';
}

onMounted(async () => {
    withLoading(async () => {
        try {
            await Promise.all([
                billingStore.getBalance(),
                billingStore.getCoupon(),
                billingStore.getCreditCards(),
                projectsStore.getProjects(),
                billingStore.getProjectUsagePriceModel(),
            ]);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.BILLING_AREA);
        }
    });

    try {
        await billingStore.getProjectUsageAndChargesCurrentRollup();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_AREA);
    } finally {
        isRollupLoading.value = false;
    }
});
</script>

<style scoped lang="scss">
.billing__new-coupon-card {
    border-width: 2px;
    border-style: dashed;
    box-shadow: none !important;
}
</style>
