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
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { Coupon } from '@/types/payments';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { useNotify, useStore } from '@/utils/hooks';

import VLoader from '@/components/common/VLoader.vue';

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const store = useStore();
const notify = useNotify();

const isCouponFetching = ref<boolean>(true);

/**
 * Returns the coupon applied to the user's account.
 */
const coupon = computed((): Coupon | null => {
    return store.state.paymentsModule.coupon;
});

/**
 * Returns the expiration date of the coupon.
 */
const expiration = computed((): string => {
    if (!coupon.value) {
        return '';
    }

    if (coupon.value?.expiresAt) {
        return 'Expires ' + coupon.value?.expiresAt.toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' });
    } else {
        return 'Unknown expiration';
    }
});

/**
 * Returns the whether the coupon is active or not.
 */
const status = computed((): string => {
    if (!coupon.value) {
        return '';
    }

    const today = new Date();
    if ((coupon.value.duration === 'forever' || coupon.value.duration === 'once') || (coupon.value.expiresAt && today.getTime() < coupon.value.expiresAt.getTime())) {
        return 'active';
    } else {
        return 'inactive';
    }
});

/**
 * Returns the whether the coupon is active or not.
 */
const expirationHelper = computed((): string => {
    if (!coupon.value) {
        return '';
    }

    switch (coupon.value.duration) {
    case 'once':
        return 'Expires after first use';
    case 'forever':
        return 'No expiration';
    default:
        return expiration.value;
    }
});

/**
 * Opens Add Coupon modal.
 */
function toggleCreateModal(): void {
    analytics.eventTriggered(AnalyticsEvent.APPLY_NEW_COUPON_CLICKED);
    store.commit(APP_STATE_MUTATIONS.TOGGLE_NEW_BILLING_ADD_COUPON_MODAL_SHOWN);
}

/**
 * Lifecycle hook after initial render.
 * Fetches coupon.
 */
onMounted(async () => {
    try {
        await store.dispatch(PAYMENTS_ACTIONS.GET_COUPON);
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.BILLING_COUPONS_TAB);
    }

    isCouponFetching.value = false;
});
</script>

<style scoped lang="scss">
    .active-discount {
        background: var(--c-green-1);
        color: var(--c-green-5);
    }

    .inactive-discount {
        background: #ffe1df;
        color: #ac1a00;
    }

    .active-status {
        background: var(--c-green-5);
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
                border: 2px dashed var(--c-grey-5);
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
                        color: var(--c-blue-3);
                        font-family: sans-serif;
                        font-size: 24px;
                    }

                    &__text {
                        color: var(--c-blue-3);
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
