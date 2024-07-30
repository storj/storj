// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive } from 'vue';
import { defineStore } from 'pinia';

import {
    AccountBalance,
    BillingInformation,
    BillingAddress,
    Coupon,
    CreditCard,
    DateRange,
    NativePaymentHistoryItem,
    PaymentHistoryPage,
    PaymentHistoryParam,
    PaymentsApi,
    PaymentStatus,
    PaymentWithConfirmations,
    ProjectCharges,
    ProjectUsagePriceModel,
    Wallet,
    TaxCountry,
    Tax,
    TaxID,
} from '@/types/payments';
import { PaymentsHttpApi } from '@/api/payments';
import { PricingPlanInfo } from '@/types/common';

export class PaymentsState {
    public balance: AccountBalance = new AccountBalance();
    public creditCards: CreditCard[] = [];
    public paymentsHistory: PaymentHistoryPage = new PaymentHistoryPage([]);
    public pendingPaymentsWithConfirmations: PaymentWithConfirmations[] = [];
    public nativePaymentsHistory: NativePaymentHistoryItem[] = [];
    public projectCharges: ProjectCharges = new ProjectCharges();
    public usagePriceModel: ProjectUsagePriceModel = new ProjectUsagePriceModel();
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

    const defaultCard = computed<CreditCard>(() => state.creditCards.find(card => card.isDefault) ?? new CreditCard());

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

    async function addTaxID(taxID: TaxID): Promise<void> {
        state.billingInformation = await api.addTaxID(taxID);
    }
    async function removeTaxID(ID: string): Promise<void> {
        state.billingInformation = await api.removeTaxID(ID);
    }

    async function getBillingInformation(): Promise<void> {
        state.billingInformation = await api.getBillingInformation();
    }

    async function saveBillingAddress(address: BillingAddress): Promise<void> {
        state.billingInformation = await api.saveBillingAddress(address);
    }

    async function getWallet(): Promise<void> {
        state.wallet = await api.getWallet();
    }

    async function claimWallet(): Promise<void> {
        state.wallet = await api.claimWallet();
    }

    async function setupAccount(): Promise<string> {
        return await api.setupAccount();
    }

    async function getCreditCards(): Promise<CreditCard[]> {
        const creditCards = await api.listCreditCards();

        state.creditCards = creditCards;

        return creditCards;
    }

    async function addCreditCard(token: string): Promise<void> {
        await api.addCreditCard(token);
    }

    async function addCardByPaymentMethodID(pmID: string): Promise<void> {
        await api.addCardByPaymentMethodID(pmID);
    }

    async function attemptPayments(): Promise<void> {
        await api.attemptPayments();
    }

    function toggleCardSelection(id: string): void {
        state.creditCards = state.creditCards.map(card => {
            if (card.id === id) {
                card.isSelected = !card.isSelected;

                return card;
            }

            card.isSelected = false;

            return card;
        });
    }

    function clearCardsSelection(): void {
        state.creditCards = state.creditCards.map(card => {
            card.isSelected = false;

            return card;
        });
    }

    function clearPendingPayments(): void {
        state.pendingPaymentsWithConfirmations = [];
    }

    async function makeCardDefault(id: string): Promise<void> {
        await api.makeCreditCardDefault(id);

        state.creditCards.forEach(card => {
            card.isDefault = card.id === id;
        });
    }

    async function removeCreditCard(cardId: string): Promise<void> {
        await api.removeCreditCard(cardId);

        state.creditCards = state.creditCards.filter(card => card.id !== cardId);
    }

    async function getPaymentsHistory(params: PaymentHistoryParam): Promise<void> {
        state.paymentsHistory = await api.paymentsHistory(params);
    }

    async function getNativePaymentsHistory(): Promise<void> {
        state.nativePaymentsHistory = await api.nativePaymentsHistory();
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

    async function getProjectUsageAndChargesCurrentRollup(): Promise<void> {
        const now = new Date();
        const endUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), now.getUTCMinutes()));
        const startUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1, 0, 0));

        state.projectCharges = await api.projectsUsageAndCharges(startUTC, endUTC);

        const dateRange = new DateRange(startUTC, endUTC);
        state.startDate = dateRange.startDate;
        state.endDate = dateRange.endDate;
    }

    async function getProjectUsageAndChargesPreviousRollup(): Promise<void> {
        const now = new Date();
        const startUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 1, 1, 0, 0));
        const endUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 0, 23, 59, 59));

        state.projectCharges = await api.projectsUsageAndCharges(startUTC, endUTC);

        const dateRange = new DateRange(startUTC, endUTC);
        state.startDate = dateRange.startDate;
        state.endDate = dateRange.endDate;
    }

    async function getProjectUsagePriceModel(): Promise<void> {
        state.usagePriceModel = await api.projectUsagePriceModel();
    }

    async function applyCouponCode(code: string): Promise<void> {
        state.coupon = await api.applyCouponCode(code);
    }

    async function getCoupon(): Promise<void> {
        state.coupon = await api.getCoupon();
    }

    async function getPricingPackageAvailable(): Promise<boolean> {
        return await api.pricingPackageAvailable();
    }

    async function purchasePricingPackage(dataStr: string, isPMID: boolean): Promise<void> {
        await api.purchasePricingPackage(dataStr, isPMID);
    }

    function setPricingPlansAvailable(available: boolean, info: PricingPlanInfo | null = null): void {
        state.pricingPlansAvailable = available;
        state.pricingPlanInfo = info;
    }

    function clear(): void {
        state.balance = new AccountBalance();
        state.creditCards = [];
        state.paymentsHistory = new PaymentHistoryPage([]);
        state.nativePaymentsHistory = [];
        state.projectCharges = new ProjectCharges();
        state.usagePriceModel = new ProjectUsagePriceModel();
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
        getBalance,
        getWallet,
        getTaxCountries,
        getCountryTaxes,
        addTaxID,
        removeTaxID,
        getBillingInformation,
        saveBillingAddress,
        claimWallet,
        setupAccount,
        getCreditCards,
        addCreditCard,
        addCardByPaymentMethodID,
        attemptPayments,
        toggleCardSelection,
        clearCardsSelection,
        makeCardDefault,
        removeCreditCard,
        getPaymentsHistory,
        getNativePaymentsHistory,
        getProjectUsageAndChargesCurrentRollup,
        getProjectUsageAndChargesPreviousRollup,
        getPaymentsWithConfirmations,
        clearPendingPayments,
        getProjectUsagePriceModel,
        applyCouponCode,
        getCoupon,
        getPricingPackageAvailable,
        purchasePricingPackage,
        setPricingPlansAvailable,
        clear,
    };
});
