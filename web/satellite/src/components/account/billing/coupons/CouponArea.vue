// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="coupon-area">
        <div class="coupon-area__top-container">
            <h1 class="coupon-area__top-container__title">Coupon</h1>
            <VButton
                v-if="couponCodeBillingUIEnabled"
                class="coupon-area__top-container__add-button"
                :on-press="onCreateClick"
                label="Add Coupon Code"
            />
        </div>
        <VLoader v-if="isCouponFetching" />
        <div v-else-if="coupon" class="coupon-area__container">
            <CouponIcon class="coupon-area__container__coupon-icon" />
            <div class="coupon-area__container__text-container">
                <div class="coupon-area__container__text-container__row">
                    <p class="coupon-area__container__text-container__row__name">{{ coupon.name }}</p>
                    <p class="coupon-area__container__text-container__row__promo">{{ coupon.promoCode }}</p>
                </div>
                <div class="coupon-area__container__text-container__row">
                    <p class="coupon-area__container__text-container__row__description">{{ coupon.getDescription() }}</p>
                </div>
                <div class="coupon-area__container__text-container__row">
                    <p class="coupon-area__container__text-container__row__expiration">
                        Active from <b>{{ startDate }}</b><template v-if="endDate"> to <b>{{ endDate }}</b></template>
                    </p>
                </div>
            </div>
        </div>
        <div v-else-if="couponCodeBillingUIEnabled" class="coupon-area__container blue">
            <CouponAddIcon class="coupon-area__container__coupon-icon" />
            <div class="coupon-area__container__text-container">
                <div class="coupon-area__container__text-container__row">
                    <p class="coupon-area__container__text-container__row__add-title">Add a Coupon to Get Started</p>
                </div>
                <div class="coupon-area__container__text-container__row">
                    <p class="coupon-area__container__text-container__row__add-subtitle">Your coupon will show up here.</p>
                </div>
            </div>
        </div>
        <div v-else class="coupon-area__container">
            <CouponIcon class="coupon-area__container__coupon-icon" />
            <div class="coupon-area__container__text-container">
                <p class="coupon-area__container__text-container__missing">No coupon is applied to your account.</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { Coupon, CouponDuration } from '@/types/payments';
import { AnalyticsHttpApi } from '@/api/analytics';

import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';

import CouponAddIcon from '@/../static/images/account/billing/couponAdd.svg';
import CouponIcon from '@/../static/images/account/billing/couponLarge.svg';

// @vue/component
@Component({
    components: {
        VButton,
        VLoader,
        CouponIcon,
        CouponAddIcon,
    },
})
export default class CouponArea extends Vue {
    public isCouponFetching = true;
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Lifecycle hook after initial render.
     * Fetches coupon.
     */
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_COUPON);
            this.isCouponFetching = false;
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Opens Add Coupon modal.
     */
    public onCreateClick(): void {
        this.analytics.pageVisit(RouteConfig.Billing.with(RouteConfig.AddCouponCode).path);
        this.$router.push({ name: RouteConfig.AddCouponCode.name });
    }

    /**
     * Returns the coupon applied to the user's account.
     */
    public get coupon(): Coupon | null {
        return this.$store.state.paymentsModule.coupon;
    }

    /**
     * Indicates if coupon code ui is enabled on the billing page.
     */
    public get couponCodeBillingUIEnabled(): boolean {
        return this.$store.state.appStateModule.couponCodeBillingUIEnabled;
    }

    /**
     * Returns the start date of the coupon.
     */
    public get startDate(): string {
        return this.coupon?.addedAt.toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' }) || '';
    }

    /**
     * Returns the expiration date of the coupon.
     */
    public get endDate(): string {
        if (!this.coupon) {
            return '';
        }

        let date: Date;

        if (this.coupon.duration == CouponDuration.Once) {
            // Last day of billing period is last day of the month
            date = new Date(this.coupon.addedAt.getFullYear(), this.coupon.addedAt.getMonth() + 1, 0);
        } else if (this.coupon.duration == CouponDuration.Repeating && this.coupon.expiresAt) {
            date = this.coupon.expiresAt;
        } else {
            return '';
        }
        
        return date.toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' });
    }

    /**
     * Returns the expiration date of the coupon.
     */
    public get expiration(): string {
        if (!this.coupon) {
            return '';
        }

        if (this.coupon.expiresAt) {
            return 'Expires ' + this.coupon.expiresAt.toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' });
        } else {
            switch (this.coupon.duration) {
            case CouponDuration.Once:
                return 'Expires after first use';
            case CouponDuration.Forever:
                return 'Never expires';
            default:
                return 'Unknown expiration';
            }
        }
    }
}
</script>

<style scoped lang="scss">
    .coupon-area {
        padding: 40px;
        margin-bottom: 32px;
        font-family: 'font_regular', sans-serif;
        font-size: 16px;
        background-color: #fff;
        border-radius: 8px;

        &__top-container {
            display: flex;
            flex-direction: row;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 28px;
                color: #384b65;
            }

            &__add-button {
                padding: 13px 16px;
            }
        }

        &__container {
            display: flex;
            flex-direction: row;
            align-items: center;
            padding: 30px 40px;
            border-radius: 6px;
            background-color: #f5f6fa;
            font-size: 16px;
            color: #61666b;

            &.blue {
                background-color: #e9f3ff;
            }

            &__coupon-icon {
                min-width: 64px;
            }

            &__text-container {
                margin-left: 40px;

                &__row {
                    display: flex;
                    flex-direction: row;
                    justify-content: flex-start;
                    margin-top: 10px;

                    &:first-child {
                        margin-top: 0;
                    }

                    &__promo {
                        margin-left: 12px;
                        color: #adadad;
                    }

                    &__description {
                        font-family: 'font_medium', sans-serif;
                        font-size: 22px;
                    }

                    &__add-title {
                        font-family: 'font_bold', sans-serif;
                        font-size: 22px;
                        color: #2683ff;
                    }

                    &__add-subtitle {
                        font-size: 14px;
                        color: #717e92;
                    }
                }
            }
        }
    }
</style>
