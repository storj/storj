// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive, ref } from 'vue';
import { defineStore } from 'pinia';

import {
    AccountBalance,
    AddCardRequest,
    AddFundsResponse,
    BillingAddress,
    BillingInformation,
    ChargeCardIntent,
    Coupon,
    CreditCard,
    DateRange,
    NativePaymentHistoryItem,
    PaymentHistoryPage,
    PaymentHistoryParam,
    PaymentsApi,
    PaymentStatus,
    PaymentWithConfirmations,
    PriceModelForPlacementRequest,
    ProductCharges,
    PurchaseRequest,
    Tax,
    TaxCountry,
    UpdateCardParams,
    UsagePriceModel,
    Wallet,
} from '@/types/payments';
import { PaymentsHttpApi } from '@/api/payments';
import { PricingPlanInfo, PricingPlanType } from '@/types/common';
import { useConfigStore } from '@/store/modules/configStore';
import { CENTS_MB_TO_DOLLARS_GB_SHIFT, centsToDollars, decimalShift, formatPrice } from '@/utils/strings';

export class PaymentsState {
    public balance: AccountBalance = new AccountBalance();
    public creditCards: CreditCard[] = [];
    public paymentsHistory: PaymentHistoryPage = new PaymentHistoryPage([]);
    public pendingPaymentsWithConfirmations: PaymentWithConfirmations[] = [];
    public nativePaymentsHistory: NativePaymentHistoryItem[] = [];
    public usagePriceModel: UsagePriceModel = new UsagePriceModel();
    public productCharges: ProductCharges = new ProductCharges();
    public startDate: Date = new Date();
    public endDate: Date = new Date();
    public coupon: Coupon | null = null;
    public wallet: Wallet = new Wallet();
    public billingInformation: BillingInformation | null = null;
    public taxCountries: TaxCountry[] = [];
    public taxes: Tax[] = [];
    public pricingPlansAvailable: boolean = false;
    public pricingPlanInfo: PricingPlanInfo | null = null;
}

export const useBillingStore = defineStore('billing', () => {
    const state = reactive<PaymentsState>(new PaymentsState());

    const api: PaymentsApi = new PaymentsHttpApi();

    const configStore = useConfigStore();
    const csrfToken = computed<string>(() => configStore.state.config.csrfToken);

    const paymentsPollingInterval = ref<number>();

    const defaultCard = computed<CreditCard>(() => state.creditCards.find(card => card.isDefault) ?? new CreditCard());

    const upgradePayUpfrontAmount = computed<number>(() => configStore.state.config.upgradePayUpfrontAmount);

    const showNewPricingTiers = computed<boolean>(() => configStore.state.config.showNewPricingTiers);

    const storagePrice = computed(() => {
        const storage =  formatPrice(decimalShift(configStore.state.config.storageMBMonthCents, CENTS_MB_TO_DOLLARS_GB_SHIFT));
        return `${storage} per GB-month`;
    });

    const egressPrice = computed(() => {
        const egress = formatPrice(decimalShift(configStore.state.config.egressMBCents, CENTS_MB_TO_DOLLARS_GB_SHIFT));
        return `${egress} per GB`;
    });

    const segmentPrice = computed(() => formatPrice(decimalShift(configStore.state.config.segmentMonthCents, 2)));

    const minimumChargeLink = '<a href="https://storj.dev/dcs/pricing#minimum-monthly-billing" target="_blank">minimum monthly usage fee</a>';

    const minimumCharge = computed(() => configStore.minimumCharge);

    const minimumChargeMsg = computed<string>(() => {
        const minimumChargeTxt = `with a ${minimumChargeLink} of ${minimumCharge.value.amount}.`;
        // even if startDate is null, priorNoticeEnabled and noticeEnabled will be false
        const isAfterStartDate = new Date() >= (minimumCharge.value.startDate ?? new Date());

        let subtitle = '';
        if (minimumCharge.value.priorNoticeEnabled) {
            subtitle += `. A ${minimumChargeLink} of ${minimumCharge.value.amount} ${isAfterStartDate ? 'applies' : 'will apply'} starting on ${minimumCharge.value.shortStartDateStr}.`;
        } else if (minimumCharge.value.isEnabled) {
            subtitle += `, ${minimumChargeTxt}`;
        } else {
            subtitle += ', no minimum';
        }

        return subtitle;
    });

    const proPlanCostInfo = computed<string>(() => {
        let minimumChargeTxt = '';

        if (minimumCharge.value.priorNoticeEnabled) {
            minimumChargeTxt += `Minimum monthly usage fee of ${minimumCharge.value.amount} applies starting ${minimumCharge.value.monthDayStartDateStr}.`;
        } else if (minimumCharge.value.isEnabled) {
            minimumChargeTxt = `Minimum of ${minimumCharge.value.amount}/month plus usage.`;
        } else {
            minimumChargeTxt = 'No minimum, billed monthly.';
        }

        minimumChargeTxt += '&nbsp;<a href="https://storj.dev/dcs/pricing" target="_blank">View pricing</a>';

        return minimumChargeTxt;
    });

    const proMinimumInfo = computed<string>(() => {
        // if (!minimumCharge.value.enabled) return '';
        let minimumChargeTxt = 'Only pay for what you use';

        if (minimumCharge.value.isEnabled) {
            minimumChargeTxt += `, with a ${minimumChargeLink} of ${minimumCharge.value.amount}.`;
        } else {
            minimumChargeTxt += '. No minimum, billed monthly.';
        }

        return minimumChargeTxt;
    });

    const proPlanInfo = computed(() => {
        const priceSummaries = configStore.state.config.productPriceSummaries ?? [];

        const payUpfrontDollars = centsToDollars(upgradePayUpfrontAmount.value);

        return new PricingPlanInfo({
            type: PricingPlanType.PRO,
            activationButtonText: upgradePayUpfrontAmount.value > 0 ? `Activate account - ${payUpfrontDollars}` : 'Activate account',
            planTitle: 'Pro Account',
            planSubtitle: 'Scale as you grow with usage based pricing',
            planCost: 'Pay-as-you-go',
            planCostInfo: proPlanCostInfo.value,
            planMinimumFeeInfo: proMinimumInfo.value,
            planUpfrontCharge: upgradePayUpfrontAmount.value > 0 ? `${payUpfrontDollars}` : '',
            planBalanceCredit: upgradePayUpfrontAmount.value > 0 ? `${payUpfrontDollars}` : '',
            planCTA: 'Start Pro Account',
            planInfo: [
                showNewPricingTiers.value ? '' : `Storage as low as ${storagePrice.value}`,
                showNewPricingTiers.value ? '' : `Download bandwidth as low as ${egressPrice.value}`,
                showNewPricingTiers.value ? '' : `Per-segment fee of ${segmentPrice.value}`,
                ...(showNewPricingTiers.value ? priceSummaries : []),
                'Set your own usage limits',
                '3 projects (+ more on request)',
                'Unlimited team members',
                'Custom domain support',
                'Priority support',
            ],
        });
    });

    async function getBalance(): Promise<AccountBalance> {
        const balance: AccountBalance = await api.getBalance();

        state.balance = balance;

        return balance;
    }

    async function getTaxCountries(): Promise<void> {
        if (!state.taxCountries.length) {
            state.taxCountries = await api.getTaxCountries();
        }
    }

    async function getCountryTaxes(countryCode: string): Promise<void> {
        state.taxes = [];
        state.taxes = await api.getCountryTaxes(countryCode);
    }

    async function addFunds(cardID: string, amount: number, intent: ChargeCardIntent): Promise<AddFundsResponse> {
        return await api.addFunds(cardID, amount, intent, csrfToken.value);
    }

    async function createIntent(amount: number, withCustomCard: boolean): Promise<string> {
        return await api.createIntent(amount, withCustomCard, csrfToken.value);
    }

    async function getCardSetupSecret(): Promise<string> {
        return await api.getCardSetupSecret();
    }

    async function addTaxID(type: string, value: string): Promise<void> {
        state.billingInformation = await api.addTaxID(type, value, csrfToken.value);
    }

    async function removeTaxID(ID: string): Promise<void> {
        state.billingInformation = await api.removeTaxID(ID, csrfToken.value);
    }

    async function addInvoiceReference(reference: string): Promise<void> {
        state.billingInformation = await api.addInvoiceReference(reference, csrfToken.value);
    }

    async function getBillingInformation(): Promise<void> {
        state.billingInformation = await api.getBillingInformation();
    }

    async function saveBillingAddress(address: BillingAddress): Promise<void> {
        state.billingInformation = await api.saveBillingAddress(address, csrfToken.value);
    }

    async function getWallet(): Promise<void> {
        state.wallet = await api.getWallet();
    }

    async function claimWallet(): Promise<void> {
        state.wallet = await api.claimWallet(csrfToken.value);
    }

    async function setupAccount(): Promise<string> {
        return await api.setupAccount(csrfToken.value);
    }

    async function getCreditCards(): Promise<CreditCard[]> {
        const creditCards = await api.listCreditCards();

        state.creditCards = creditCards;

        return creditCards;
    }

    async function updateCreditCard(params: UpdateCardParams): Promise<void> {
        await api.updateCreditCard(params, csrfToken.value);
    }

    async function addCardByPaymentMethodID(request: AddCardRequest): Promise<void> {
        await api.addCardByPaymentMethodID(request, csrfToken.value);
    }

    async function attemptPayments(): Promise<void> {
        await api.attemptPayments(csrfToken.value);
    }

    async function makeCardDefault(id: string): Promise<void> {
        await api.makeCreditCardDefault(id, csrfToken.value);

        state.creditCards.forEach(card => {
            card.isDefault = card.id === id;
        });
    }

    async function removeCreditCard(cardId: string): Promise<void> {
        await api.removeCreditCard(cardId, csrfToken.value);

        state.creditCards = state.creditCards.filter(card => card.id !== cardId);
    }

    async function getPaymentsHistory(params: PaymentHistoryParam): Promise<void> {
        state.paymentsHistory = await api.paymentsHistory(params);
    }

    async function getNativePaymentsHistory(): Promise<void> {
        state.nativePaymentsHistory = await api.nativePaymentsHistory();
    }

    function startPaymentsPolling(): void {
        if (paymentsPollingInterval.value) return;

        paymentsPollingInterval.value = window.setInterval(() => {
            getPaymentsWithConfirmations().catch(_ => {});
        }, 30000);
    }

    function stopPaymentsPolling(): void {
        clearInterval(paymentsPollingInterval.value);
    }

    async function getPaymentsWithConfirmations(): Promise<void> {
        const newPayments = await api.paymentsWithConfirmations();
        newPayments.forEach(newPayment => {
            const oldPayment = state.pendingPaymentsWithConfirmations.find(old => old.transaction === newPayment.transaction);

            if (newPayment.status === PaymentStatus.Pending) {
                if (oldPayment) {
                    oldPayment.confirmations = newPayment.confirmations;
                    return;
                }

                state.pendingPaymentsWithConfirmations.push(newPayment);
                return;
            }

            if (oldPayment) oldPayment.status = PaymentStatus.Confirmed;
        });
    }

    async function getProductUsageAndChargesCurrentRollup(): Promise<void> {
        const now = new Date();
        const endUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), now.getUTCMinutes()));
        const startUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1, 0, 0));

        state.productCharges = await api.productsUsageAndCharges(startUTC, endUTC);

        const dateRange = new DateRange(startUTC, endUTC);
        state.startDate = dateRange.startDate;
        state.endDate = dateRange.endDate;
    }

    async function getProjectUsagePriceModel(): Promise<void> {
        state.usagePriceModel = await api.projectUsagePriceModel();
    }

    async function getPriceModelForPlacement(params: PriceModelForPlacementRequest): Promise<UsagePriceModel> {
        return await api.getPlacementPriceModel(params);
    }

    async function applyCouponCode(code: string): Promise<void> {
        state.coupon = await api.applyCouponCode(code, csrfToken.value);
    }

    async function getCoupon(): Promise<void> {
        state.coupon = await api.getCoupon();
    }

    async function getPricingPackageAvailable(): Promise<boolean> {
        return await api.pricingPackageAvailable();
    }

    async function purchasePricingPackage(request: PurchaseRequest): Promise<void> {
        await api.purchase(request, csrfToken.value);
    }

    async function purchaseUpgradedAccount(request: PurchaseRequest): Promise<void> {
        await api.purchase(request, csrfToken.value);
    }

    function setPricingPlansAvailable(available: boolean, info: PricingPlanInfo | null = null): void {
        if (info) {
            info.planMinimumFeeInfo = `After the discount is used or expires, continue with pay-as-you-go.`;
            info.planMinimumFeeInfo += '&nbsp;<a href="https://storj.dev/dcs/pricing" target="_blank">View pricing</a>';
            if (minimumCharge.value.isEnabled)
                info.planInfo.push('$5 minimum monthly usage after validity period');
        }
        state.pricingPlansAvailable = available;
        state.pricingPlanInfo = info;
    }

    async function startFreeTrial(): Promise<void> {
        await api.startFreeTrial(csrfToken.value);
    }

    function clear(): void {
        stopPaymentsPolling();
        paymentsPollingInterval.value = undefined;
        state.balance = new AccountBalance();
        state.creditCards = [];
        state.paymentsHistory = new PaymentHistoryPage([]);
        state.nativePaymentsHistory = [];
        state.usagePriceModel = new UsagePriceModel();
        state.productCharges = new ProductCharges();
        state.pendingPaymentsWithConfirmations = [];
        state.startDate = new Date();
        state.endDate = new Date();
        state.coupon = null;
        state.wallet = new Wallet();
        state.billingInformation = null;
        state.pricingPlansAvailable = false;
        state.pricingPlanInfo = null;
    }

    return {
        state,
        defaultCard,
        proPlanInfo,
        minimumChargeMsg,
        storagePrice,
        egressPrice,
        segmentPrice,
        upgradePayUpfrontAmount,
        getBalance,
        getWallet,
        getTaxCountries,
        getCountryTaxes,
        addTaxID,
        addFunds,
        createIntent,
        getCardSetupSecret,
        removeTaxID,
        getBillingInformation,
        addInvoiceReference,
        saveBillingAddress,
        claimWallet,
        setupAccount,
        getCreditCards,
        updateCreditCard,
        addCardByPaymentMethodID,
        attemptPayments,
        makeCardDefault,
        removeCreditCard,
        getPaymentsHistory,
        getNativePaymentsHistory,
        getProductUsageAndChargesCurrentRollup,
        startPaymentsPolling,
        stopPaymentsPolling,
        getProjectUsagePriceModel,
        getPriceModelForPlacement,
        applyCouponCode,
        getCoupon,
        getPricingPackageAvailable,
        purchasePricingPackage,
        purchaseUpgradedAccount,
        setPricingPlansAvailable,
        startFreeTrial,
        clear,
    };
});
