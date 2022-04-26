// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <form id="payment-form">
        <div class="form-row">
            <div id="card-element">
                <!-- A Stripe Element will be inserted here. -->
            </div>
            <div id="card-errors" role="alert" />
        </div>
    </form>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { MetaUtils } from '@/utils/meta';

interface StripeResponse {
    error: string
    token: {
        id: unknown
        card: {
            funding : string
        }
    }
}

// StripeCardInput encapsulates Stripe add card addition logic
// @vue/component
@Component
export default class StripeCardInput extends Vue {
    @Prop({default: () => console.error('onStripeResponse is not reinitialized')})
    private readonly onStripeResponseCallback: (tokenId: unknown) => void;

    private isLoading = false;

    /**
     * Stripe elements is using to create 'Add Card' form.
     */
    private cardElement: any; // eslint-disable-line @typescript-eslint/no-explicit-any

    /**
     * Stripe library.
     */
    private stripe: any; // eslint-disable-line @typescript-eslint/no-explicit-any

    /**
     * Stripe initialization after initial render.
     */
    public async mounted(): Promise<void> {
        if (!window['Stripe']) {
            await this.$notify.error('Stripe library not loaded');

            return;
        }

        const stripePublicKey = MetaUtils.getMetaContent('stripe-public-key');

        this.stripe = window['Stripe'](stripePublicKey);

        if (!this.stripe) {
            await this.$notify.error('Unable to initialize stripe');

            return;
        }

        const elements = this.stripe.elements();

        if (!elements) {
            await this.$notify.error('Unable to instantiate elements');

            return;
        }

        this.cardElement = elements.create('card');

        if (!this.cardElement) {
            await this.$notify.error('Unable to create card');

            return;
        }

        this.cardElement.mount('#card-element');
        this.cardElement.addEventListener('change', function (event): void {
            const displayError: HTMLElement = document.getElementById('card-errors') as HTMLElement;
            if (event.error) {
                displayError.textContent = event.error.message;
            } else {
                displayError.textContent = '';
            }
        });
    }

    /**
     * Event after card adding.
     * Returns token to callback and clears card input
     *
     * @param result stripe response
     */
    public async onStripeResponse(result: StripeResponse): Promise<void> {
        if (result.error) {
            throw result.error;
        }

        if (result.token.card.funding === 'prepaid') {
            await this.$notify.error('Prepaid cards are not supported');

            return;
        }

        await this.onStripeResponseCallback(result.token.id);
        this.cardElement.clear();
    }

    /**
     * Clears listeners.
     */
    public beforeDestroy(): void {
        this.cardElement.removeEventListener('change');
    }

    /**
     * Fires stripe event after all inputs are filled.
     */
    public async onSubmit(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        try {
            await this.stripe.createToken(this.cardElement).then(this.onStripeResponse);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.isLoading = false;
    }
}
</script>

<style scoped lang="scss">
    .StripeElement {
        box-sizing: border-box;
        width: 100%;
        padding: 13px 12px;
        border: 1px solid transparent;
        border-radius: 4px;
        background-color: white;
        box-shadow: 0 1px 3px 0 #e6ebf1;
        -webkit-transition: box-shadow 150ms ease;
        transition: box-shadow 150ms ease;
    }

    .StripeElement--focus {
        box-shadow: 0 1px 3px 0 #cfd7df;
    }

    .StripeElement--invalid {
        border-color: #fa755a;
    }

    .StripeElement--webkit-autofill {
        background-color: #fefde5 !important;
    }

    .form-row {
        width: 100%;
    }
</style>
