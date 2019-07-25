// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

export async function setupStripe(context: any, resultCallback: (result: any) => any) {
    if (!window['Stripe']) {
        context.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Stripe library not loaded');

        return;
    }

    const stripe = window['Stripe'](process.env.VUE_APP_STRIPE_PUBLIC_KEY);
    if (!stripe) {
        context.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to initialize stripe');

        return;
    }

    const elements = stripe.elements();
    if (!elements) {
        context.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to instantiate elements');

        return;
    }

    const card = elements.create('card');
    if (!card) {
        context.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to create card');

        return;
    }

    card.mount('#card-element');

    card.addEventListener('change', function (event) {
        const displayError = document.getElementById('card-errors') as HTMLElement;
        if (event.error) {
            displayError.textContent = event.error.message;
        } else {
            displayError.textContent = '';
        }
    });

    const form = document.getElementById('payment-form') as HTMLElement;
    form.addEventListener('submit', function (event) {
        event.preventDefault();
        stripe.createToken(card).then(async function (result: any) {
            if (result.token.card.funding == 'prepaid') {
                context.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Prepaid cards are not supported');

                return;
            }

            await resultCallback(result);

            card.clear();
        });
    });
}
