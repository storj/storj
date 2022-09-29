// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="coupon-area">
        <div class="coupon-area__top-container">
            <h1 class="coupon-area__title">Coupon</h1>
            <VLoader v-if="isCouponFetching" />
            <div class="coupon-area__container">
                <div
                    v-if="coupon"
                    class="coupon-area__container__existing-coupons"
                >
                    <div class="coupon-area__container__existing-coupons__discount-top-container ">
                        <span :class="`coupon-area__container__existing-coupons__discount-top-container__discount ${status}-discount`">
                            {{ coupon.getDescription().slice(0, coupon.getDescription().indexOf(' ')) }}
                        </span>
                    </div>
                    <div class="coupon-area__container__existing-coupons__status-container">
                        <span :class="`coupon-area__container__existing-coupons__status-container__status ${status}-status`">
                            {{ status }}
                        </span>
                    </div>
                    <div class="coupon-area__container__existing-coupons__discount-black-container">
                        <span class="coupon-area__container__existing-coupons__discount-black-container__discount">
                            {{ coupon.getDescription().slice(0, coupon.getDescription().indexOf(' ')) }} off
                        </span>
                    </div>
                    <div class="coupon-area__container__existing-coupons__expiration-container">
                        <span class="coupon-area__container__existing-coupons__expiration-container__expiration">
                            {{ expirationHelper }}
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
            <AddCouponCode2
                v-if="showCreateCode"
                @toggleMethod="toggleCreateModal"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { Coupon } from '@/types/payments';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import AddCouponCode2 from '@/components/account/billing/coupons/AddCouponCode2.vue';
import VLoader from '@/components/common/VLoader.vue';

// @vue/component
@Component({
    components: {
        VLoader,
        AddCouponCode2,
    },
})
export default class Coupons extends Vue {
    public isCouponFetching = true;
    public showCreateCode = false;

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
            this.isCouponFetching = false;
        }
    }

    /**
     * Opens Add Coupon modal.
     */
    public toggleCreateModal(): void {
        this.analytics.eventTriggered(AnalyticsEvent.APPLY_NEW_COUPON_CLICKED);
        this.showCreateCode = !this.showCreateCode;
    }

    /**
     * Returns the coupon applied to the user's account.
     */
    public get coupon(): Coupon | null {
        return this.$store.state.paymentsModule.coupon;
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
            return 'Unknown expiration';
        }
    }

    /**
     * Returns the whether the coupon is active or not.
     */
    public get status(): string {
        if (!this.coupon) {
            return '';
        }

        const today = new Date();
        if ((this.coupon.duration === 'forever' || this.coupon.duration === 'once') || (this.coupon.expiresAt && today.getTime() < this.coupon.expiresAt.getTime())) {
            return 'active';
        } else {
            return 'inactive';
        }
    }

    /**
     * Returns the whether the coupon is active or not.
     */
    public get expirationHelper(): string {
        if (!this.coupon) {
            return '';
        }

        switch (this.coupon.duration) {
        case 'once':
            return 'Expires after first use';
        case 'forever':
            return 'No expiration';
        default:
            return this.expiration;
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
            gap: 10px;

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
                        min-width: 50px;
                        border-radius: 10px;
                        display: flex;
                        align-items: center;
                        justify-content: center;
                        padding: 0 8px;
                        font-family: sans-serif;
                        font-weight: 700;
                        font-size: 16px;
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
                        text-transform: capitalize;
                    }
                }

                &__discount-black-container {
                    grid-column: 1;
                    grid-row: 2;

                    &__discount {
                        font-family: sans-serif;
                        font-weight: 1000;
                        font-size: 22px;
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

    @media only screen and (max-width: 768px) {

        .coupon-area__container {
            flex-direction: column;

            &__existing-coupons {
                max-width: unset;
                width: 90%;

                &__discount-black-container {
                    margin-top: 24px;
                }
            }

            &__new-coupon {
                max-width: 100%;
                width: 90%;
            }
        }
    }
</style>
