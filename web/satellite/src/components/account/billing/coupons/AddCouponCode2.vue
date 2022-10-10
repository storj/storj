// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-coupon__wrapper">
        <div class="add-coupon__modal">
            <div class="add-coupon__container">
                <div class="add-coupon__header-wrapper">
                    <CloseIcon
                        class="add-coupon__close-icon"
                        @click="onCloseClick"
                    />
                </div>
                <div class="add-coupon__body-wrapper">
                    <CouponIcon />
                    <h2 class="add-coupon__body-wrapper__title">Apply Coupon Code</h2>
                    <p class="add-coupon__body-wrapper__text">If you have a coupon active, it will automatically be replaced.</p>
                </div>
                <AddCouponCodeInput2 @close="onCloseClick" />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import AddCouponCodeInput2 from '@/components/common/AddCouponCodeInput2.vue';

import CloseIcon from '@/../static/images/common/closeCross.svg';
import CouponIcon from '@/../static/images/account/billing/greenCoupon.svg';

// @vue/component
@Component({
    components: {
        CloseIcon,
        AddCouponCodeInput2,
        CouponIcon,
    },
})
export default class AddCouponCode2 extends Vue {

    @Prop({ default: false })
    protected readonly success: boolean;
    @Prop({ default: false })
    protected readonly error: boolean;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
    * Closes add coupon modal.
    */
    public onCloseClick(): void {
        this.analytics.eventTriggered(AnalyticsEvent.COUPON_CODE_APPLIED);
        this.$emit('toggleMethod');
    }

}
</script>

<style scoped lang="scss">
    .add-coupon {

        &__wrapper {
            background: #1b2533c7 75%;
            position: fixed;
            width: 100%;
            height: 100%;
            top: 0;
            left: 0;
        }

        &__modal {
            width: 40%;
            min-width: 300px;
            max-width: 390px;
            aspect-ratio: 1/1;
            height: auto;
            background: #fff;
            border-radius: 8px;
            margin: 15% auto;
            position: relative;
            padding-bottom: 20px;
        }

        &__container {
            width: 85%;
            padding-top: 10px;
            margin: 0 auto;
        }

        &__header-wrapper {
            display: flex;
            justify-content: right;
            padding-bottom: 20px;
        }

        &__body-wrapper {
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-direction: column;
            height: 150px;

            &__title {
                font-family: sans-serif;
                font-weight: 800;
                text-align: center;
            }

            &__text {
                font-family: sans-serif;
                text-align: center;
            }
        }

        &__close-icon {
            position: relative;
            top: 17px;
            cursor: pointer;
        }
    }

</style>
