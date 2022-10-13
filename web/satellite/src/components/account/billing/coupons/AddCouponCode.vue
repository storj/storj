// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-coupon__wrapper">
        <div class="add-coupon__modal">
            <div class="add-coupon__container">
                <div class="add-coupon__header-wrapper">
                    <p class="add-coupon__header">Add Coupon Code</p>
                    <CloseIcon
                        class="add-coupon__close-icon"
                        @click="onCloseClick"
                    />
                </div>
                <AddCouponCodeInput />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { AnalyticsHttpApi } from '@/api/analytics';

import AddCouponCodeInput from '@/components/common/AddCouponCodeInput.vue';

import CloseIcon from '@/../static/images/common/closeCross.svg';

// @vue/component
@Component({
    components: {
        CloseIcon,
        AddCouponCodeInput,
    },
})
export default class AddCouponCode extends Vue {

    @Prop({ default: false })
    protected readonly success: boolean;
    @Prop({ default: false })
    protected readonly error: boolean;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
    * Closes add coupon modal.
    */
    public onCloseClick(): void {
        this.analytics.pageVisit(RouteConfig.Account.with(RouteConfig.Billing).path);
        this.$router.push(RouteConfig.Account.with(RouteConfig.Billing).path);
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
            width: 741px;
            height: 298px;
            background: #fff;
            border-radius: 8px;
            margin: 15% auto;
            position: relative;
        }

        &__container {
            width: 85%;
            padding-top: 10px;
            margin: 0 auto;
        }

        &__header-wrapper {
            display: flex;
            justify-content: space-between;
        }

        &__header {
            font-family: 'font_bold', sans-serif;
            font-style: normal;
            font-weight: bold;
            font-size: 16px;
            line-height: 148.31%;
            margin: 15px 0;
            display: inline-block;
        }

        &__close-icon {
            position: relative;
            top: 17px;
            cursor: pointer;
        }
    }

</style>
