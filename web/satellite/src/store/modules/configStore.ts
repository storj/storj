// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive, ref } from 'vue';
import { defineStore } from 'pinia';

import {
    BrandingConfig,
    createDefaultBranding,
    defaultBrandingName,
    FrontendConfig,
    FrontendConfigApi,
    LogoKey,
} from '@/types/config';
import { FrontendConfigHttpApi } from '@/api/config';
import { centsToDollars } from '@/utils/strings';
import { Time } from '@/utils/time';
import { User } from '@/types/users';
import { PricingPlanInfo } from '@/types/common';
import { APIError } from '@/utils/error';

export class ConfigState {
    public config: FrontendConfig = new FrontendConfig();
    public branding: BrandingConfig = createDefaultBranding();
}

export const useConfigStore = defineStore('config', () => {
    const state = reactive<ConfigState>(new ConfigState());

    const configApi: FrontendConfigApi = new FrontendConfigHttpApi();

    const minimumCharge = computed<MinimumCharge>(() => {
        if (!state.config.minimumCharge) {
            return new MinimumCharge();
        }
        return new MinimumCharge(
            state.config.minimumCharge.enabled,
            state.config.minimumCharge.amount,
            state.config.minimumCharge.startDate,
        );
    });

    const brandName = computed<string>(() => state.branding.name);
    const supportUrl = computed<string>(() => state.branding.supportUrl);
    const docsUrl = computed<string>(() => state.branding.docsUrl);
    const homepageUrl = computed<string>(() => state.branding.homepageUrl);
    const isDefaultBrand = computed<boolean>(() => brandName.value === defaultBrandingName);
    const logo = computed<string>(() => state.branding.getLogo(LogoKey.FullLight) ?? '');
    const darkLogo = computed<string>(() => state.branding.getLogo(LogoKey.FullDark) ?? '');

    const billingEnabled = computed<boolean>(() => state.config.billingFeaturesEnabled && isDefaultBrand.value);

    const minimumChargeBannerDismissed = ref(false);

    async function getConfig(): Promise<FrontendConfig> {
        const result = await configApi.get();

        state.config = result;

        return result;
    }

    async function getBranding(): Promise<BrandingConfig> {
        const result = await configApi.getBranding();

        state.branding = result;

        return result;
    }

    function setFallbackBranding(cfg: FrontendConfig): void {
        state.branding.getInTouchUrl = cfg.scheduleMeetingURL;
        state.branding.supportUrl = cfg.generalRequestURL;
        state.branding.homepageUrl = cfg.homepageURL;
        state.branding.docsUrl = cfg.documentationURL;
    }

    const signupConfig = ref<Map<string, unknown>>(new Map());
    const onboardingConfig = ref<Map<string, unknown>>(new Map());

    async function getPartnerSignupConfig(partner: string): Promise<void> {
        if (!partner || signupConfig.value.has(partner)) return;

        try {
            const conf = await configApi.getPartnerUIConfig('signup', partner);
            signupConfig.value.set(partner, conf);
        } catch (error) {
            if (error instanceof APIError && error.status !== 404) {
                throw error;
            }
            try {
                const config = (await import('@/configs/registrationViewConfig.json')).default;
                if (!config[partner]) return;
                signupConfig.value.set(partner, config[partner]);
            } catch { /* empty */ }
        }
    }

    async function getPartnerOnboardingConfig(partner: string): Promise<void> {
        if (!partner || onboardingConfig.value.has(partner)) return;

        try {
            const conf = await configApi.getPartnerUIConfig('onboarding', partner);
            onboardingConfig.value.set(partner, conf);
        } catch (error) {
            if (error instanceof APIError && error.status !== 404) {
                throw error;
            }
            try {
                const config = (await import('@/configs/onboardingConfig.json')).default;
                if (!config[partner]) return;
                onboardingConfig.value.set(partner, config[partner]);
            } catch { /* empty */ }
        }
    }

    async function getPartnerPricingPlanConfig(partner: string): Promise<PricingPlanInfo | null> {
        if (!partner) return null;
        try {
            return (await configApi.getPartnerUIConfig('pricing-plan', partner)) as PricingPlanInfo;
        } catch (error) {
            if (error instanceof APIError && error.status !== 404) {
                throw error;
            }
            try {
                const config = (await import('@/configs/pricingPlanConfig.json')).default;
                return (config[partner] as PricingPlanInfo);
            } catch {
                return null;
            }
        }
    }

    function getBillingEnabled(user: User): boolean {
        return billingEnabled.value && !user.hasVarPartner && !user.isNFR;
    }

    /**
     * Determines if a project has the new pricing based on its creation date.
     * @param projectCreatedAt
     */
    function getProjectHasNewPricing(projectCreatedAt: string | null): boolean {
        if (!projectCreatedAt) {
            return false;
        }
        if (!state.config.newPricingStartDate) {
            return false;
        }
        const projectCreatedDate = new Date(projectCreatedAt);
        const newPricingDate = new Date(state.config.newPricingStartDate);
        return projectCreatedDate >= newPricingDate;
    }

    return {
        state,
        minimumCharge,
        minimumChargeBannerDismissed,
        signupConfig,
        onboardingConfig,
        brandName,
        supportUrl,
        docsUrl,
        homepageUrl,
        isDefaultBrand,
        logo,
        darkLogo,
        billingEnabled,
        getConfig,
        getBranding,
        getPartnerSignupConfig,
        getPartnerOnboardingConfig,
        getPartnerPricingPlanConfig,
        getBillingEnabled,
        getProjectHasNewPricing,
        setFallbackBranding,
    };
});

/**
 * MinimumCharge represents minimum charge config.
 */
export class MinimumCharge {
    public constructor(
        public enabled = false,
        public _amount = 0,
        public _startDate: string | null = null,
    ) { }

    get amount(): string {
        return centsToDollars(this._amount);
    }

    get startDate(): Date | null {
        return this._startDate !== null ? new Date(this._startDate) : null;
    }

    get monthYearStartDateStr(): string {
        if (!this.startDate) {
            return '';
        }
        return Time.formattedDate(this.startDate, { month: 'long', year: 'numeric', timeZone: 'UTC' });
    }

    get monthDayStartDateStr(): string {
        if (!this.startDate) {
            return '';
        }
        return Time.formattedDate(this.startDate, { month: 'long', day: 'numeric', timeZone: 'UTC' });
    }

    get shortStartDateStr(): string {
        if (!this.startDate) {
            return '';
        }
        const str = Time.formattedDate(this.startDate, { month: 'long', day: 'numeric', timeZone: 'UTC', timeZoneName: 'short' });
        const parts = str.split(' at');
        return parts.join('');
    }

    get longStartDateStr(): string {
        if (!this.startDate) {
            return '';
        }
        const str = Time.formattedDate(this.startDate, { month: 'long', day: 'numeric', year: 'numeric', timeZone: 'UTC', timeZoneName: 'short' });
        const parts = str.split(' at');
        return parts.join('');
    }

    // notice is enabled 60 days before the start date and 45 days after the start date.
    get priorNoticeEnabled(): boolean {
        if (!this.enabled || !this.startDate) return false;

        const currentDate = new Date();
        const startDate = this.startDate;
        const sixtyDaysBefore = new Date(startDate);
        sixtyDaysBefore.setDate(sixtyDaysBefore.getDate() - 60);
        const forty5DaysAfter = new Date(startDate);
        forty5DaysAfter.setDate(forty5DaysAfter.getDate() + 45);

        return currentDate >= sixtyDaysBefore && currentDate <= forty5DaysAfter;
    }

    // indicates whether minimum charge is fully enabled.
    get isEnabled(): boolean {
        if (!this.enabled) return false;
        return this.startDate === null || new Date() >= this.startDate;
    }
}
