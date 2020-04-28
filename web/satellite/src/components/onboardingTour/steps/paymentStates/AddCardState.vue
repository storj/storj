// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-card-state">
        <div class="add-card-state__input-container">
            <StripeCardInput
                ref="stripeCardInput"
                :on-stripe-response-callback="addCard"
            />
            <div class="add-card-state__input-container__blur" v-if="isLoading"/>
        </div>
        <div class="add-card-state__security-info">
            <LockImage/>
            <span class="add-card-state__security-info__text">
                Your card is secured by 128-bit SSL and AES-256 encryption. Your information is secure.
            </span>
        </div>
        <div class="add-card-state__button" :class="{ loading: isLoading }" @click="onConfirmAddStripe">
            <img
                v-if="isLoading"
                class="add-card-state__button__loading-image"
                src="@/../static/images/account/billing/loading.gif"
                alt="loading gif"
            >
            <span class="add-card-state__button__label">{{ isLoading ? 'Adding' : 'Add Payment' }}</span>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';

import LockImage from '@/../static/images/account/billing/lock.svg';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

const {
    ADD_CREDIT_CARD,
    GET_CREDIT_CARDS,
    GET_BILLING_HISTORY,
    GET_BALANCE,
} = PAYMENTS_ACTIONS;

interface StripeForm {
    onSubmit(): Promise<void>;
}

@Component({
    components: {
        StripeCardInput,
        LockImage,
    },
})

export default class AddCardState extends Vue {
    public isLoading: boolean = false;

    public $refs!: {
        stripeCardInput: StripeCardInput & StripeForm;
    };

    /**
     * Provides card information to Stripe.
     */
    public async onConfirmAddStripe(): Promise<void> {
        await this.$refs.stripeCardInput.onSubmit();

        this.$segment.track(SegmentEvent.PAYMENT_METHOD_ADDED, {
            project_id: this.$store.getters.selectedProject.id,
        });
    }

    /**
     * Adds card after Stripe confirmation.
     *
     * @param token from Stripe
     */
    public async addCard(token: string) {
        if (this.isLoading) return;

        this.isLoading = true;
        this.$emit('toggleIsLoading');

        try {
            await this.$store.dispatch(ADD_CREDIT_CARD, token);
        } catch (error) {
            await this.$notify.error(error.message);
            this.setDefaultState();

            return;
        }

        await this.$notify.success('Card successfully added');
        try {
            await this.$store.dispatch(GET_CREDIT_CARDS);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.setDefaultState();

        this.$emit('setProjectState');
    }

    /**
     * Sets area to default state.
     */
    private setDefaultState(): void {
        this.isLoading = false;
        this.$emit('toggleIsLoading');
    }
}
</script>

<style scoped lang="scss">
    .add-card-state {
        font-family: 'font_regular', sans-serif;
        width: 100%;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: space-between;

        &__input-container {
            position: relative;
            width: calc(100% - 90px);
            padding: 25px 45px 35px 45px;
            background-color: #fff;

            &__blur {
                position: absolute;
                top: 0;
                left: 0;
                height: 100%;
                width: 100%;
                background-color: rgba(229, 229, 229, 0.2);
                z-index: 100;
            }
        }

        &__security-info {
            width: calc(100% - 70px);
            padding: 15px 35px;
            background-color: #cef0e3;
            border-radius: 0 0 8px 8px;
            margin-bottom: 45px;
            display: flex;
            align-items: center;
            justify-content: center;

            &__text {
                margin-left: 5px;
                font-size: 15px;
                line-height: 18px;
                color: #1a9666;
            }
        }

        &__button {
            display: flex;
            justify-content: center;
            align-items: center;
            width: 156px;
            height: 48px;
            cursor: pointer;
            border-radius: 6px;
            background-color: #2683ff;

            &__loading-image {
                margin-right: 5px;
                width: 18px;
                height: 18px;
            }

            &__label {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 23px;
                color: #fff;
                word-break: keep-all;
                white-space: nowrap;
            }

            &:hover {
                background-color: #0059d0;
            }
        }
    }

    .loading {
        opacity: 0.6;
        pointer-events: none;
    }
</style>