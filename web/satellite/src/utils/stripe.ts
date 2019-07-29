// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Stripe contains
export default class Stripe {
    private cardElement: any;

    private stripe: any;

    private createCardCallback: (result: any) => any;

    public newCardInput(resultCallback: (result: any) => any) {
        this.createCardCallback = resultCallback;

        if (!window['Stripe']) {
            throw new Error('Stripe library not loaded');
        }

        this.stripe = window['Stripe'](process.env.VUE_APP_STRIPE_PUBLIC_KEY);
        if (!this.stripe) {
            throw new Error('Unable to initialize stripe');
        }

        const elements = this.stripe.elements();
        if (!elements) {
            throw new Error('Unable to instantiate elements');
        }

        this.cardElement = elements.create('card');
        if (!this.cardElement) {
            throw new Error('Unable to create card');
        }

        this.cardElement.mount('#card-element');

        this.cardElement.addEventListener('change', function (event) {
            const displayError: HTMLElement = document.getElementById('card-errors') as HTMLElement;
            if (event.error) {
                displayError.textContent = event.error.message;
            } else {
                displayError.textContent = '';
            }
        });

        const form: HTMLElement = document.getElementById('payment-form') as HTMLElement;
        form.addEventListener('submit', this.onSubmitEventListener);
    }

    private onSubmitEventListener = (event) => {
        event.preventDefault();
        this.stripe.createToken(this.cardElement).then(this.onStripeResponse.bind(this));
    }

    private async onStripeResponse(result: any) {
        if (result.token.card.funding == 'prepaid') {
            throw new Error('Prepaid cards are not supported');
        }

        await this.createCardCallback(result);

        this.cardElement.clear();
    }
}
