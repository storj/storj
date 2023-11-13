// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="onCloseClick">
        <template #content>
            <div class="modal">
                <CouponIcon />
                <h2 class="modal__title">Apply Coupon Code</h2>
                <p class="modal__text">Entering a new coupon will replace any existing coupon.</p>
                <NewBillingAddCouponCodeInput @close="onCloseClick" />
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAppStore } from '@/store/modules/appStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import NewBillingAddCouponCodeInput from '@/components/common/NewBillingAddCouponCodeInput.vue';
import VModal from '@/components/common/VModal.vue';

import CouponIcon from '@/../static/images/account/billing/greenCoupon.svg';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();

/**
 * Closes add coupon modal.
 */
function onCloseClick(): void {
    analyticsStore.eventTriggered(AnalyticsEvent.COUPON_CODE_APPLIED);
    appStore.removeActiveModal();
}
</script>

<style scoped lang="scss">
    .modal {
        width: 500px;
        padding: 32px;
        font-family: 'font_regular', sans-serif;

        @media screen and (width <= 650px) {
            width: unset;
            padding: 24px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 148.31%;
            margin: 15px 0;
        }

        &__text {
            text-align: center;
        }
    }
</style>
