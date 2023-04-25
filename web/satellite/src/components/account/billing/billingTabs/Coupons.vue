// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="coupon-area">
        <h1 class="coupon-area__title">Coupons</h1>
        <VLoader v-if="isCouponFetching" />
        <div class="coupon-area__wrapper">
            <div v-if="coupon" class="coupon-area__wrapper__coupon">
                <div class="coupon-area__wrapper__coupon__icon" :class="{'expired': !isActive}">
                    <CloudIcon v-if="coupon.partnered" />
                    <CouponIcon v-else />
                </div>
                <h2 class="coupon-area__wrapper__coupon__title">{{ coupon.name }}</h2>
                <p class="coupon-area__wrapper__coupon__price">{{ discount }}</p>
                <p class="coupon-area__wrapper__coupon__status" :class="{'expired': !isActive}">{{ isActive ? 'Active' : 'Expired' }}</p>
                <p class="coupon-area__wrapper__coupon__expiration">{{ expiration }}</p>
            </div>
            <div
                v-if="couponCodeBillingUIEnabled"
                class="coupon-area__wrapper__add-coupon"
                @click="toggleCreateModal"
            >
                <span class="coupon-area__wrapper__add-coupon__plus-icon">+&nbsp;</span>
                <span class="coupon-area__wrapper__add-coupon__text">Apply New Coupon</span>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { Coupon, CouponDuration } from '@/types/payments';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';

import VLoader from '@/components/common/VLoader.vue';

import CouponIcon from '@/../static/images/billing/coupon.svg';
import CloudIcon from '@/../static/images/onboardingTour/cloudIcon.svg';

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const configStore = useConfigStore();
const appStore = useAppStore();
const billingStore = useBillingStore();
const notify = useNotify();

const isCouponFetching = ref<boolean>(true);

/**
 * Returns the coupon applied to the user's account.
 */
const coupon = computed((): Coupon | null => {
    return billingStore.state.coupon;
});

/**
 * Returns the expiration date of the coupon.
 */
const expiration = computed((): string => {
    const c = coupon.value;
    if (!c) return '';

    const exp = c.expiresAt;
    if (!exp || c.duration === CouponDuration.Forever) {
        return 'No expiration';
    }
    return `Expires on ${exp.getDate()} ${SHORT_MONTHS_NAMES[exp.getMonth()]} ${exp.getFullYear()}`;
});

/**
 * Returns the coupon's discount amount.
 */
const discount = computed((): string => {
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
const isActive = computed((): boolean => {
    const now = Date.now();
    const c = coupon.value;
    return !!c && (c.duration === 'forever' || (!!c.expiresAt && now < c.expiresAt.getTime()));
});

/**
 * Returns the whether applying a new coupon is enabled.
 */
const couponCodeBillingUIEnabled = computed((): boolean => {
    return configStore.state.config.couponCodeBillingUIEnabled;
});

/**
 * Opens Add Coupon modal.
 */
function toggleCreateModal(): void {
    if (!couponCodeBillingUIEnabled) {
        return;
    }
    analytics.eventTriggered(AnalyticsEvent.APPLY_NEW_COUPON_CLICKED);
    appStore.updateActiveModal(MODALS.newBillingAddCoupon);
}

/**
 * Lifecycle hook after initial render.
 * Fetches coupon.
 */
onMounted(async () => {
    try {
        await billingStore.getCoupon();
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.BILLING_COUPONS_TAB);
    }

    isCouponFetching.value = false;
});
</script>

<style scoped lang="scss">
    .coupon-area {
        margin-top: 16px;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 18px;
            line-height: 27px;
        }

        &__wrapper {
            margin-top: 19px;
            display: flex;
            gap: 16px;

            &__coupon {
                width: 340px;
                box-sizing: border-box;
                padding: 24px;
                display: flex;
                flex-direction: column;
                align-items: flex-start;
                gap: 8px;
                border-radius: 20px;
                background-color: var(--c-white);

                &__icon {
                    width: 40px;
                    height: 40px;
                    box-sizing: border-box;
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    border-radius: 10px;
                    background-color: rgba(0 172 38 / 10%);

                    :deep(path) {
                        fill: var(--c-green-5);
                    }

                    &.expired {
                        background-color: rgb(255 138 0 / 10%);

                        :deep(path) {
                            fill: var(--c-yellow-5);
                        }
                    }
                }

                &__title {
                    font-family: 'font_bold', sans-serif;
                    font-size: 24px;
                    line-height: 31px;
                }

                &__price {
                    font-family: 'font_medium', sans-serif;
                    font-size: 14px;
                    line-height: 20px;
                }

                &__status {
                    box-sizing: border-box;
                    padding: 4px 11px;
                    background-color: var(--c-green-5);
                    border-radius: 4px;
                    font-family: 'font_medium', sans-serif;
                    font-size: 12px;
                    color: var(--c-white);

                    &.expired {
                        background-color: var(--c-yellow-5);
                    }
                }

                &__expiration {
                    font-family: 'font_regular', sans-serif;
                    font-size: 14px;
                    line-height: 20px;
                    color: var(--c-grey-6);
                }
            }

            &__add-coupon {
                width: 340px;
                min-height: 213px;
                display: flex;
                align-items: center;
                justify-content: center;
                border: 2px dashed var(--c-grey-5);
                border-radius: 20px;
                color: var(--c-blue-3);
                font-family: 'font_regular', sans-serif;
                cursor: pointer;

                &:hover {
                    background-color: var(--c-blue-1);
                }

                &__plus-icon {
                    font-size: 24px;
                }

                &__text {
                    font-size: 18px;
                    text-decoration: underline;
                }
            }
        }
    }

    @media only screen and (max-width: 768px) {

        .coupon-area__wrapper {
            flex-direction: column;

            &__coupon,
            &__add-coupon {
                width: unset;
            }
        }
    }
</style>
