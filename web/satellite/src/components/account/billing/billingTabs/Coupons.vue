// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="coupon-area">
        <div class="coupon-area__top-container">
            <h1 class="coupon-area__title">coupons</h1>
            <VLoader v-if="isCouponFetching" />
            <div class="coupon-area__container">
                <div
                    v-if="coupon"
                    class="coupon-area__container__existing-coupons"
                >
                    <div class="coupon-area__container__existing-coupons__discount-top-container active">
                        <span class="coupon-area__container__existing-coupons__discount-top-container active">
                            ${{ coupon.getDescription() }}
                        </span>
                    </div>
                    <div class="coupon-area__container__existing-coupons__status-container">
                        <span class="coupon-area__container__existing-coupons__discount-top-container active">
                            Active
                        </span>
                    </div>
                    <div class="coupon-area__container__existing-coupons__discount-black-container">
                        <span class="coupon-area__container__existing-coupons__discount-black-container__discount">
                            ${{ coupon.getDescription() }} off
                        </span>
                    </div>
                    <div class="coupon-area__container__existing-coupons__expiration-container">
                        <span class="coupon-area__container__existing-coupons__expiration-container__text">
                            Expiration in {{ expiration }}
                        </span>
                    </div>
                </div>
                <div 
                    class="coupon-area__container__new-coupon"
                    @click="toggleCreateModal"
                >
                    <div class="coupon-area__container__new-coupon__text-area">
                        <span class="coupon-area__container__new-coupon__text-area__plus-icon">+&nbsp;</span>
                        <span class="coupon-area__container__new-coupon__text-area__text">Apply New Coupon</span>
                    </div>
                </div>
            </div>
            <AddCoupon2 
                v-if="showCreateCode"
                @toggleMethod="toggleCreateModal"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VLoader from '@/components/common/VLoader.vue';

import AddCoupon2 from '@/components/account/billing/coupons/AddCouponCode2.vue'

import { RouteConfig } from '@/router';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { Coupon, CouponDuration } from '@/types/payments';

// @vue/component
@Component({
    components: {
        VLoader,
        AddCoupon2,
    },
})
export default class CouponArea extends Vue {
    public isCouponFetching = true;
    public showCreateCode = false;

    public testData = [{index: 1, status: "Active", discount: 3.25, expiration: 'Jul 2023'},{index: 2, status: "Inactive", discount: 1000, expiration: 'Mar 2023'},{index: 3, status: "Inactive", discount: 28.95, expiration: 'Jan 2023'}]

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
        this.$router.push(RouteConfig.Billing.with(RouteConfig.AddCouponCode).path);
    }

    /**
     * Opens Add Coupon modal.
     */
    public toggleCreateModal(): void {
        this.showCreateCode = !this.showCreateCode
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
    .active-discount {
        background: #dffff7;
        color: #00ac26;
    }

    .inactive-discount {
        background: #ffe1df;
        color: #ac1a00;
    }

    .active-status {
        background: #00ac26;
    }

    .inactive-status {
        background: #ac1a00;
    }

    .coupon-area {

        &__title {
            font-family: sans-serif;
            font-size: 24px;
            margin: 20px 0;
        }

        &__container {
            display: flex;
            flex-wrap: wrap;

            &__existing-coupons {
                border-radius: 10px;
                max-width: 400px;
                width: 18vw;
                min-width: 227px;
                max-height: 222px;
                height: 10vw;
                min-height: 126px;
                display: grid;
                grid-template-columns: 4fr 1fr;
                grid-template-rows: 2fr 1fr 1fr;
                margin: 0 10px 10px 0;
                padding: 20px;
                box-shadow: 0 0 20px rgb(0 0 0 / 4%);
                background: #fff;

                &__discount-top-container {
                    grid-column: 1;
                    grid-row: 1;
                    margin: 0 0 auto;

                    &__discount {
                        height: 60px;
                        width: fit-content;
                        min-width: 60px;
                        border-radius: 10px;
                        display: flex;
                        align-items: center;
                        justify-content: center;
                        padding: 0 8px;
                        font-family: sans-serif;
                        font-weight: 700;
                        font-size: 18px;
                    }
                }

                &__status-container {
                    grid-column: 2;
                    grid-row: 1;
                    margin: 0 0 0 auto;

                    &__status {
                        height: 30px;
                        width: 50px;
                        border-radius: 5px;
                        display: flex;
                        align-items: center;
                        justify-content: center;
                        padding: 0 8px;
                        font-family: sans-serif;
                        font-weight: 700;
                        font-size: 14px;
                        color: #fff;
                    }
                }

                &__discount-black-container {
                    grid-column: 1;
                    grid-row: 2;

                    &__discount {
                        font-family: sans-serif;
                        font-weight: 1000;
                        font-size: 28px;
                    }
                }

                &__expiration-container {
                    grid-column: 1;
                    grid-row: 3;

                    &__expiration {
                        font-family: sans-serif;
                        font-weight: 500;
                        font-size: 14px;
                    }
                }
            }

            &__new-coupon {
                border: 2px dashed #929fb1;
                border-radius: 10px;
                max-width: 400px;
                width: 18vw;
                min-width: 227px;
                max-height: 222px;
                height: 10vw;
                min-height: 126px;
                padding: 18px;
                display: flex;
                align-items: center;
                justify-content: center;
                cursor: pointer;

                &__text-area {
                    display: flex;
                    align-items: center;
                    justify-content: center;

                    &__plus-icon {
                        color: #0149ff;
                        font-family: sans-serif;
                        font-size: 24px;
                    }

                    &__text {
                        color: #0149ff;
                        font-family: sans-serif;
                        font-size: 18px;
                        text-decoration: underline;
                    }
                }
            }
        }
    }
</style>
