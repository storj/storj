// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <low-token-balance-banner
            v-if="isLowBalance"
            :cta-label="tab !== TABS['payment-methods'] ? 'Deposit' : ''"
            @click="onAddTokensClicked"
        />

        <v-row>
            <v-col>
                <PageTitleComponent title="Account Billing" />
            </v-col>
        </v-row>

        <v-card color="default" class="mt-2 mb-6">
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
                <v-tab v-if="billingInformationUIEnabled">
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
                        >
                            <template #loader>
                                <v-progress-linear v-if="isLoading" indeterminate />
                            </template>
                            <v-card-text>
                                <v-chip rounded color="success" variant="tonal" class="font-weight-bold mb-2">
                                    {{ centsToDollars(priceSummary) }}
                                </v-chip>
                                <v-divider class="my-4" />
                                <v-btn variant="outlined" color="default" size="small" class="mr-2" @click="tab = TABS['billing-history']">View Billing History</v-btn>
                            </v-card-text>
                        </v-card>
                    </v-col>

                    <v-col cols="12" sm="4">
                        <v-card title="Account Balance" subtitle="Your STORJ account balance" variant="flat">
                            <template #loader>
                                <v-progress-linear v-if="isLoading" indeterminate />
                            </template>
                            <v-card-text>
                                <v-chip rounded color="success" variant="tonal" class="font-weight-bold mb-2">
                                    {{ formattedAccountBalance }}
                                </v-chip>
                                <v-divider class="my-4" />
                                <v-btn variant="outlined" color="default" size="small" class="mr-2" :prepend-icon="mdiPlus" @click="onAddTokensClicked">
                                    Add STORJ Tokens
                                </v-btn>
                            </v-card-text>
                        </v-card>
                    </v-col>

                    <v-col cols="12" sm="4">
                        <v-card
                            v-if="isLoading"
                            class="d-flex align-center justify-center"
                            height="200"
                            variant="flat"
                        >
                            <template #loader>
                                <v-progress-linear v-if="isLoading" indeterminate />
                            </template>
                        </v-card>
                        <v-card
                            v-else-if="coupon"
                            :title="`Coupon / ${coupon.name}`"
                            height="100%"
                            :subtitle="`${isCouponActive ? 'Active' : 'Expired'} / ${couponExpiration}`"
                            variant="flat"
                        >
                            <v-card-text>
                                <v-chip
                                    :color="isCouponActive ? 'success' : 'error'"
                                    variant="tonal"
                                    class="font-weight-bold mb-2"
                                    rounded
                                >
                                    {{ couponDiscount }}
                                </v-chip>

                                <v-divider class="my-4" />

                                <v-btn
                                    v-if="couponCodeBillingUIEnabled"
                                    variant="outlined"
                                    color="default"
                                    size="small"
                                    class="mr-2"
                                    :prepend-icon="mdiPlus"
                                    @click="isAddCouponDialogShown = true"
                                >
                                    Add Coupon
                                </v-btn>
                            </v-card-text>
                        </v-card>

                        <v-card
                            v-else-if="couponCodeBillingUIEnabled"
                            title="Coupon"
                            subtitle="Apply a new coupon to your account"
                            variant="flat"
                        >
                            <v-card-text>
                                <v-chip rounded color="default" variant="tonal" class="font-weight-bold mb-2">
                                    No Coupon
                                </v-chip>

                                <v-divider class="my-4" />

                                <v-btn
                                    variant="outlined"
                                    color="default"
                                    size="small"
                                    class="mr-2"
                                    :prepend-icon="mdiPlus"
                                    @click="isAddCouponDialogShown = true"
                                >
                                    Apply New Coupon
                                </v-btn>
                            </v-card-text>
                        </v-card>
                    </v-col>
                </v-row>

                <v-row>
                    <v-col>
                        <v-card title="Detailed Usage Report" subtitle="Get a complete usage report for all your projects." variant="flat">
                            <v-card-text>
                                <v-btn variant="outlined" color="default" size="small" :prepend-icon="mdiCalendar">
                                    <detailed-usage-report-dialog />
                                    Download Report
                                </v-btn>
                            </v-card-text>
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
                    <v-col cols="12" md="4" sm="6">
                        <StorjTokenCardComponent ref="tokenCardComponent" @historyClicked="tab = TABS.transactions" />
                    </v-col>

                    <v-col v-for="(card, i) in creditCards" :key="i" cols="12" md="4" sm="6">
                        <CreditCardComponent :card="card" />
                    </v-col>

                    <v-col cols="12" md="4" sm="6">
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

            <v-window-item v-if="billingInformationUIEnabled">
                <billing-information-tab />
            </v-window-item>
        </v-window>
    </v-container>

    <apply-coupon-code-dialog v-model="isAddCouponDialogShown" />
</template>

<script setup lang="ts">
import { computed, onBeforeMount, onMounted, ref } from 'vue';
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
    VProgressLinear,
} from 'vuetify/components';
import { useRoute, useRouter } from 'vue-router';
import { mdiCalendar, mdiPlus } from '@mdi/js';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { AccountBalance, Coupon, CouponDuration, CreditCard } from '@/types/payments';
import { centsToDollars } from '@/utils/strings';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useLowTokenBalance } from '@/composables/useLowTokenBalance';
import { ROUTES } from '@/router';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import CreditCardComponent from '@/components/CreditCardComponent.vue';
import AddCreditCardComponent from '@/components/AddCreditCardComponent.vue';
import BillingHistoryTab from '@/components/billing/BillingHistoryTab.vue';
import UsageAndChargesComponent from '@/components/billing/UsageAndChargesComponent.vue';
import StorjTokenCardComponent from '@/components/StorjTokenCardComponent.vue';
import TokenTransactionsTableComponent from '@/components/TokenTransactionsTableComponent.vue';
import ApplyCouponCodeDialog from '@/components/dialogs/ApplyCouponCodeDialog.vue';
import LowTokenBalanceBanner from '@/components/LowTokenBalanceBanner.vue';
import DetailedUsageReportDialog from '@/components/dialogs/DetailedUsageReportDialog.vue';
import BillingInformationTab from '@/components/billing/BillingInformationTab.vue';

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

const billingStore = useBillingStore();
const projectsStore = useProjectsStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();
const appStore = useAppStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();
const router = useRouter();
const route = useRoute();
const isLowBalance = useLowTokenBalance();

const isRollupLoading = ref(true);
const isAddCouponDialogShown = ref<boolean>(false);

const tokenCardComponent = ref<IStorjTokenCardComponent>();

const creditCards = computed((): CreditCard[] => {
    return billingStore.state.creditCards;
});

const couponCodeBillingUIEnabled = computed<boolean>(() => configStore.state.config.couponCodeBillingUIEnabled);
const billingInformationUIEnabled = computed<boolean>(() => configStore.state.config.billingInformationTabEnabled);

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
 * Returns account balance (sum storjscan and stripe credit) from store.
 */
const formattedAccountBalance = computed((): string => {
    return billingStore.state.balance.formattedSum;
});

/**
 * Returns the coupon applied to the user's account.
 */
const coupon = computed((): Coupon | null => {
    return billingStore.state.coupon;
});

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

function onAddTokensClicked(): void {
    if (!usersStore.state.user.paidTier) {
        appStore.toggleUpgradeFlow(true);
        return;
    }

    tab.value = TABS['payment-methods'];
    tokenCardComponent.value?.onAddTokens();
}

onBeforeMount(() => {
    if (!configStore.getBillingEnabled(usersStore.state.user.hasVarPartner)) {
        router.replace({ name: ROUTES.AccountSettings.name });
    }
});

onMounted(async () => {
    withLoading(async () => {
        const promises: Promise<void | AccountBalance | CreditCard[]>[] = [
            billingStore.getBalance(),
            billingStore.getCoupon(),
            billingStore.getCreditCards(),
            billingStore.getProjectUsagePriceModel(),
        ];

        if (configStore.state.config.nativeTokenPaymentsEnabled) {
            promises.push(billingStore.getNativePaymentsHistory());
        }

        try {
            await Promise.all(promises);
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
