// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import OverviewStep from '@/components/onboardingTour/steps/OverviewStep.vue';

import { PaymentsHttpApi } from '@/api/payments';
import { router } from '@/router';
import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { mount } from '@vue/test-utils';

const paymentsApi = new PaymentsHttpApi();
const paymentsModule = makePaymentsModule(paymentsApi);

const store = new Vuex.Store({ modules: { paymentsModule }});

describe('OverviewStep.vue', (): void => {
    it('renders correctly', async (): Promise<void> => {
        const wrapper = mount(OverviewStep, {
            router,
            store,
        });

        await store.commit(PAYMENTS_MUTATIONS.SET_PAYWALL_ENABLED_STATUS, true);

        expect(wrapper).toMatchSnapshot();

        await store.commit(PAYMENTS_MUTATIONS.SET_PAYWALL_ENABLED_STATUS, false);

        expect(wrapper).toMatchSnapshot();
    });
});
