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

        <v-card color="default" class="mt-2 mb-6" rounded="md">
            <v-tabs
                v-model="tab"
                color="primary"
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
            <v-window-item class="pb-2">
                <v-row>
                    <v-col cols="12" sm="12" md="6" lg="6" xl="4">
                        <v-card
                            :subtitle="estimatedChargesSubtitle"
                            class="pa-2"
                        >
                            <template #title>
                                <v-row class="align-center">
                                    <v-col>
                                        <span>Estimated Total Cost</span>
                                        <span class="ml-2">
                                            <v-icon class="text-cursor-pointer" size="14" :icon="Info" color="info" />
                                            <v-tooltip
                                                class="text-center"
                                                activator="parent"
                                                location="top"
                                                max-width="450"
                                            >
                                                {{ estimatedChargesTooltipMsg }}
                                            </v-tooltip>
                                        </span>
                                    </v-col>
                                </v-row>
                            </template>
                            <template #loader>
                                <v-progress-linear v-if="isLoading" indeterminate />
                            </template>
                            <v-card-text>
                                <div class="d-flex align-center">
                                    <span class="text-h5 font-weight-bold">{{ estimatedChargesValue }}</span>
                                </div>
                                <v-divider class="my-4 border-0" />
                                <v-btn variant="outlined" color="default" rounded="md" class="mr-2" :append-icon="ArrowRight" @click="tab = TABS['billing-history']">View Billing History</v-btn>
                            </v-card-text>
                        </v-card>
                    </v-col>

                    <v-col cols="12" sm="12" md="6" lg="6" xl="4">
                        <v-card subtitle="Your Storj account balance" class="pa-2">
                            <template #title>
                                <v-row class="align-center">
                                    <v-col>
                                        <span>Available Funds</span>
                                        <span class="ml-2">
                                            <v-icon class="text-cursor-pointer" size="14" :icon="Info" color="info" />
                                            <v-tooltip
                                                class="text-center"
                                                activator="parent"
                                                location="top"
                                            >
                                                Prepaid balance for upcoming account usage.
                                            </v-tooltip>
                                        </span>
                                    </v-col>
                                </v-row>
                            </template>
                            <template #loader>
                                <v-progress-linear v-if="isLoading" indeterminate />
                            </template>
                            <v-card-text>
                                <div class="d-flex align-center">
                                    <span class="text-h5 font-weight-bold">{{ formattedAccountBalance }}</span>
                                </div>
                                <v-divider class="my-4 border-0" />
                                <div v-if="checkoutEnabled" class="d-inline-block mr-2">
                                    <v-btn
                                        variant="outlined"
                                        color="default"
                                        :disabled="!creditCards.length && !checkoutEnabled"
                                        :prepend-icon="Plus"
                                        @click="isAddFundsDialogShown = true"
                                    >
                                        Add Funds
                                    </v-btn>
                                    <v-tooltip
                                        v-if="!creditCards.length && !checkoutEnabled"
                                        class="text-center"
                                        activator="parent"
                                        location="top"
                                    >
                                        Please add a credit card first to proceed with adding funds.
                                    </v-tooltip>
                                </div>
                                <v-btn
                                    variant="outlined"
                                    color="default"
                                    class="mr-2"
                                    :prepend-icon="Plus"
                                    @click="onAddTokensClicked"
                                >
                                    Add STORJ Tokens
                                </v-btn>
                            </v-card-text>
                        </v-card>
                    </v-col>

                    <v-col cols="12" sm="12" md="6" lg="6" xl="4">
                        <v-card
                            v-if="isLoading"
                            class="d-flex align-center justify-center pa-2"
                            height="200"
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
                            class="pa-2"
                        >
                            <v-card-text>
                                <v-chip
                                    :color="isCouponActive ? 'success' : 'error'"
                                    variant="tonal"
                                    class="font-weight-bold"
                                >
                                    {{ couponDiscount }}
                                </v-chip>

                                <v-divider class="my-4 border-0" />

                                <v-btn
                                    v-if="couponCodeBillingUIEnabled"
                                    variant="outlined"
                                    color="default"
                                    :prepend-icon="Plus"
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
                            height="100%"
                            class="pa-2"
                        >
                            <v-card-text>
                                <v-chip color="default" variant="tonal" class="text-caption">
                                    No Coupon
                                </v-chip>

                                <v-divider class="my-4 border-0" />

                                <v-btn
                                    variant="outlined"
                                    color="default"
                                    :prepend-icon="Plus"
                                    @click="isAddCouponDialogShown = true"
                                >
                                    Apply New Coupon
                                </v-btn>
                            </v-card-text>
                        </v-card>
                    </v-col>

                    <v-col cols="12" sm="12" md="6" lg="6" xl="4">
                        <v-card title="Detailed Usage Report" subtitle="Get complete report of usage for your account" class="pa-2" height="100%">
                            <v-card-text>
                                <v-chip color="default" variant="tonal" class="text-caption">
                                    All Projects
                                </v-chip>
                                <v-divider class="my-4 border-0" />
                                <v-btn variant="outlined" color="default" rounded="md" :prepend-icon="Calendar">
                                    <detailed-usage-report-dialog />
                                    Detailed Usage Report
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

            <v-window-item class="pb-2">
                <v-row>
                    <v-col cols="12" sm="12" md="6" lg="6" xl="4">
                        <StorjTokenCardComponent ref="tokenCardComponent" @history-clicked="tab = TABS.transactions" />
                    </v-col>

                    <v-col v-for="card in creditCards" :key="card.id" cols="12" sm="12" md="6" lg="6" xl="4">
                        <CreditCardComponent :card="card" />
                    </v-col>

                    <v-col cols="12" sm="12" md="6" lg="6" xl="4">
                        <AddCreditCardComponent />
                    </v-col>
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

    <apply-coupon-code-dialog v-model="isAddCouponDialogShown" />
    <add-funds-dialog v-if="checkoutEnabled" v-model="isAddFundsDialogShown" />
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
    VTooltip,
    VIcon,
} from 'vuetify/components';
import { useRoute, useRouter } from 'vue-router';
import { Calendar, Info, Plus, ArrowRight } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useBillingStore } from '@/store/modules/billingStore';
import { AccountBalance, Coupon, CouponDuration, CreditCard } from '@/types/payments';
import { centsToDollars } from '@/utils/strings';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { MinimumCharge, useConfigStore } from '@/store/modules/configStore';
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
import AddFundsDialog from '@/components/dialogs/AddFundsDialog.vue';

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
const isAddFundsDialogShown = ref<boolean>(false);

const tokenCardComponent = ref<IStorjTokenCardComponent>();

const creditCards = computed((): CreditCard[] =>
    billingStore.state.creditCards
        .slice()
        .sort((a, b) => Number(b.isDefault) - Number(a.isDefault)),
);

const couponCodeBillingUIEnabled = computed<boolean>(() => configStore.state.config.couponCodeBillingUIEnabled);
const billingInformationUIEnabled = computed<boolean>(() => configStore.state.config.billingInformationTabEnabled);
const checkoutEnabled = computed<boolean>(() => configStore.state.config.billingStripeCheckoutEnabled);
const minimumChargeCfg = computed<MinimumCharge>(() => configStore.minimumCharge);

/**
 * projectIDs is an array of all of the project IDs for which there exist project usage charges.
 */
const projectIDs = computed((): string[] => {
    return projectsStore.state.projects
        .filter(proj => productCharges.value.hasProject(proj.id))
        .sort((proj1, proj2) => proj1.name.localeCompare(proj2.name))
        .map(proj => proj.id);
});

const userPaidTier = computed<boolean>(() => usersStore.state.user.isPaid);

const willMinimumChargeBeApplied = computed(() => {
    const { isEnabled, _amount } = minimumChargeCfg.value;

    return isEnabled &&
        userPaidTier.value &&
        productCharges.value.applyMinimumCharge &&
        priceSummary.value > 0 &&
        priceSummary.value < _amount;
});

const estimatedChargesSubtitle = computed<string>(() => {
    const date = `${new Date().toLocaleString('en-US', { month: 'long', year: 'numeric' })}`;

    if (willMinimumChargeBeApplied.value) {
        return `${date} = ${centsToDollars(priceSummary.value)} usage, ${centsToDollars(minimumChargeCfg.value._amount)} minimum`;
    }

    return date;
});

const estimatedChargesValue = computed<string>(() => {
    if (willMinimumChargeBeApplied.value) {
        return centsToDollars(minimumChargeCfg.value._amount);
    }

    return centsToDollars(priceSummary.value);
});

const estimatedChargesTooltipMsg = computed<string>(() => {
    if (willMinimumChargeBeApplied.value) {
        const minimumAmount = centsToDollars(minimumChargeCfg.value._amount);

        return `Storj has a ${minimumAmount} monthly minimum. Since your usage (${centsToDollars(priceSummary.value)})
            is below this amount, you'll be charged the minimum. Once your usage exceeds ${minimumAmount},
            you'll only pay for what you use.`;
    }

    return 'Estimated charges for current billing period.';
});

/**
 * Returns the product charges from the billing store.
 */
const productCharges = computed(() => billingStore.state.productCharges);

/**
 * Returns price summary of all project usages.
 */
const priceSummary = computed((): number => productCharges.value.getPrice());

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
    if (!userPaidTier.value) {
        appStore.toggleUpgradeFlow(true);
        return;
    }

    tab.value = TABS['payment-methods'];
    tokenCardComponent.value?.onAddTokens();
}

onBeforeMount(() => {
    if (!configStore.getBillingEnabled(usersStore.state.user)) {
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
        await billingStore.getProductUsageAndChargesCurrentRollup();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_AREA);
    } finally {
        isRollupLoading.value = false;
    }
});
</script>
