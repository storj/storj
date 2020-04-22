// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-card-area">
        <p class="add-card-area__label">Add Credit or Debit Card</p>
        <StripeCardInput
            class="add-card-area__stripe"
            ref="stripeCardInput"
            :on-stripe-response-callback="addCard"
        />
        <div class="add-card-area__submit-area"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { ProjectOwning } from '@/utils/projectOwning';

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
    },
})
export default class AddCardForm extends Vue {
    public $refs!: {
        stripeCardInput: StripeCardInput & StripeForm;
    };

    /**
     * Adds card after Stripe confirmation.
     *
     * @param token from Stripe
     */
    public async addCard(token: string) {
        this.$emit('toggleIsLoading');

        try {
            await this.$store.dispatch(ADD_CREDIT_CARD, token);
        } catch (error) {
            await this.$notify.error(error.message);

            this.$emit('toggleIsLoading');

            return;
        }

        await this.$notify.success('Card successfully added');
        this.$segment.track(SegmentEvent.PAYMENT_METHOD_ADDED, {
            project_id: this.$store.getters.selectedProject.id,
        });
        try {
            await this.$store.dispatch(GET_CREDIT_CARDS);
        } catch (error) {
            await this.$notify.error(error.message);
            this.$emit('toggleIsLoading');
        }

        this.$emit('toggleIsLoading');
        this.$emit('toggleIsLoaded');

        if (!this.userHasOwnProject) {
            await this.$store.dispatch(APP_STATE_ACTIONS.SHOW_CREATE_PROJECT_BUTTON);
        }

        setTimeout(() => {
            this.$emit('cancel');
            this.$emit('toggleIsLoaded');

            setTimeout(() => {
                if (!this.userHasOwnProject) {
                    this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_PROJ);
                }
            }, 500);
        }, 2000);
    }

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
     * Indicates if user has own project.
     */
    private get userHasOwnProject(): boolean {
        return new ProjectOwning(this.$store).userHasOwnProject();
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
