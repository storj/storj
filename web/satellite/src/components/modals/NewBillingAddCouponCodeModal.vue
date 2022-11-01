// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="onCloseClick">
        <template #content>
            <div class="modal">
                <CouponIcon />
                <h2 class="modal__title">Apply Coupon Code</h2>
                <p class="modal__text">If you have a coupon active, it will automatically be replaced.</p>
                <NewBillingAddCouponCodeInput @close="onCloseClick" />
            </div>
        </template>
    </VModal>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import NewBillingAddCouponCodeInput from '@/components/common/NewBillingAddCouponCodeInput.vue';
import VModal from '@/components/common/VModal.vue';

import CouponIcon from '@/../static/images/account/billing/greenCoupon.svg';

// @vue/component
@Component({
    components: {
        VModal,
        NewBillingAddCouponCodeInput,
        CouponIcon,
    },
})
export default class NewBillingAddCouponCodeModal extends Vue {
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
    * Closes add coupon modal.
    */
    public onCloseClick(): void {
        this.analytics.eventTriggered(AnalyticsEvent.COUPON_CODE_APPLIED);
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_NEW_BILLING_ADD_COUPON_MODAL_SHOWN);
    }
}
</script>

<style scoped lang="scss">
    .modal {
        width: 500px;
        padding: 32px;
        font-family: 'font_regular', sans-serif;

        @media screen and (max-width: 650px) {
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
