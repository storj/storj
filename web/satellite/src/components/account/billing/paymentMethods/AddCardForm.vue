// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-card-area">
        <p class="add-card-area__label">Add Credit or Debit Card</p>
        <StripeCardInput
            ref="stripeCardInput"
            class="add-card-area__stripe"
            :on-stripe-response-callback="addCard"
        />
        <div class="add-card-area__submit-area" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { USER_ACTIONS } from '@/store/modules/users';
import { AnalyticsHttpApi } from '@/api/analytics';

import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';

const {
    ADD_CREDIT_CARD,
    GET_CREDIT_CARDS,
} = PAYMENTS_ACTIONS;

interface StripeForm {
    onSubmit(): Promise<void>;
}

// @vue/component
@Component({
    components: {
        StripeCardInput,
    },
})
export default class AddCardForm extends Vue {
    public $refs!: {
        stripeCardInput: StripeCardInput & StripeForm;
    };

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Adds card after Stripe confirmation.
     *
     * @param token from Stripe
     */
    public async addCard(token: string): Promise<void> {
        this.$emit('toggleIsLoading');

        try {
            await this.$store.dispatch(ADD_CREDIT_CARD, token);

            // We fetch User one more time to update their Paid Tier status.
            await this.$store.dispatch(USER_ACTIONS.GET);
        } catch (error) {
            await this.$notify.error(error.message);

            this.$emit('toggleIsLoading');

            return;
        }

        await this.$notify.success('Card successfully added');
        try {
            await this.$store.dispatch(GET_CREDIT_CARDS);
        } catch (error) {
            await this.$notify.error(error.message);
            this.$emit('toggleIsLoading');
        }

        this.$emit('toggleIsLoading');
        this.$emit('toggleIsLoaded');

        setTimeout(() => {
            this.$emit('cancel');
            this.$emit('toggleIsLoaded');

            setTimeout(() => {
                if (!this.userHasOwnProject) {
                    this.analytics.pageVisit(RouteConfig.CreateProject.path);
                    this.$router.push(RouteConfig.CreateProject.path);
                }
            }, 500);
        }, 2000);
    }

    /**
     * Provides card information to Stripe.
     */
    public async onConfirmAddStripe(): Promise<void> {
        await this.$refs.stripeCardInput.onSubmit();
    }

    /**
     * Indicates if user has own project.
     */
    private get userHasOwnProject(): boolean {
        return this.$store.getters.projectsCount > 0;
    }
}
</script>

<style scoped lang="scss">
    .add-card-area {
        margin-top: 44px;
        display: flex;
        max-height: 52px;
        justify-content: space-between;
        align-items: center;
        font-family: 'font_regular', sans-serif;

        &__label {
            font-family: 'font_medium', sans-serif;
            font-size: 21px;
        }

        &__stripe {
            width: 60%;
            min-width: 400px;
        }

        &__submit-area {
            display: flex;
            align-items: center;
            min-width: 135px;
        }
    }
</style>
