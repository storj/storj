// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="credit-history__coupon-modal-wrapper">
        <div class="credit-history__coupon-modal">
            <div class="credit-history__coupon-modal__container">
                <div class="credit-history__coupon-modal__header-wrapper">
                    <p class="credit-history__coupon-modal__header">Add Coupon Code</p>
                    <CloseIcon
                        class="credit-history__coupon-modal__close-icon"
                        @click="onCloseClick"
                    />
                </div>
                <div class="credit-history__coupon-modal__input-wrapper">
                    <img
                        class="credit-history__coupon-modal__input-icon"
                        src="@/../static/images/account/billing/coupon.png"
                        alt="Coupon"
                    />
                    <HeaderlessInput
                        placeholder="Enter Coupon Code"
                        class="credit-history__coupon-modal__input"
                        width="92% !important"
                        height="52px"
                        :withIcon="true"
                    />
                    <CheckIcon
                        class="credit-history__coupon-modal__check"
                        v-if="success"
                    />
                    <VButton
                        label="Claim Coupon"
                        class="credit-history__coupon-modal__claim-button"
                        width="133px"
                        height="32px"
                        v-if="!success"
                    />
                </div>
                <SuccessMessage
                    class="credit-history__coupon-modal__success-message"
                    message="Get 50GB free and start using Tardigrade today."
                    v-if="success"
                />
                <VButton
                    class="credit-history__coupon-modal__apply-button"
                    label="Apply Coupon Code"
                    width="85%"
                    height="44px"
                />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import SuccessMessage from '@/components/common/SuccessMessage.vue';
import VButton from '@/components/common/VButton.vue';

import CloseIcon from '@/../static/images/common/closeCross.svg';
import CheckIcon from '@/../static/images/common/success-check.svg';

import { RouteConfig } from '@/router';

@Component({
    components: {
        VButton,
        HeaderlessInput,
        CloseIcon,
        CheckIcon,
        SuccessMessage,
    },
})
export default class AddCoupon extends Vue {

    @Prop({default: false})
    protected readonly success: boolean;

    /**
     * Closes add coupon modal.
     */
    public onCloseClick(): void {
        this.$router.push(RouteConfig.Account.with(RouteConfig.Billing).path);
    }

}
</script>

<style scoped lang="scss">

    .credit-history {

        &__coupon-modal-wrapper {
            background: #1b2533c7 75%;
            position: fixed;
            width: 100%;
            height: 100%;
            top: 0;
            left: 0;
            z-index: 1000;
        }

        &__coupon-modal {
            width: 741px;
            height: 298px;
            background: #fff;
            border-radius: 8px;
            margin: 15% auto;
            position: relative;

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

            &__input-wrapper {
                position: relative;
            }

            &__claim-button {
                position: absolute;
                right: 20px;
                bottom: 11px;
                font-size: 14px;
                padding: 0 5px;
            }

            &__success-message {
                position: relative;
                top: 15px;
            }

            &__apply-button {
                position: absolute;
                left: 0;
                right: 0;
                margin: 0 auto;
                bottom: 40px;
                background: #93a1af;

                &:hover {
                    background: darken(#93a1af, 10%);
                }
            }

            &__check {
                position: absolute;
                right: 15px;
                bottom: 15px;
            }

            &__input-icon {
                position: absolute;
                top: 28px;
                z-index: 1;
                left: 25px;
                width: 25.01px;
                height: 16.67px;
            }
        }
    }

</style>
