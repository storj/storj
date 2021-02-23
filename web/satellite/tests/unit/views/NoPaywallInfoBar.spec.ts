// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import NoPaywallInfoBar from '@/components/noPaywallInfoBar/NoPaywallInfoBar.vue';

import { router } from '@/router';
import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { PaymentsHistoryItem, PaymentsHistoryItemType, ProjectUsageAndCharges } from '@/types/payments';
import { createLocalVue, mount } from '@vue/test-utils';

import { PaymentsMock } from '../mock/api/payments';

const localVue = createLocalVue();
const paymentsModule = makePaymentsModule(new PaymentsMock());
const { SET_PAYMENTS_HISTORY } = PAYMENTS_MUTATIONS;
const store = new Vuex.Store({
    modules: {
        paymentsModule,
    },
});
localVue.use(Vuex);

describe('NoPaywallInfoBar.vue', () => {
    it('renders correctly with less than 75% usage', () => {
        const coupon: PaymentsHistoryItem = new PaymentsHistoryItem('id', 'coupon', 300, 300, 'test', '', new Date(), new Date(), PaymentsHistoryItemType.Coupon, 275);
        store.commit(SET_PAYMENTS_HISTORY, [coupon]);

        const wrapper = mount(NoPaywallInfoBar, {
            store,
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with more than 75% usage', () => {
        const coupon: PaymentsHistoryItem = new PaymentsHistoryItem('id', 'coupon', 300, 300, 'test', '', new Date(), new Date(), PaymentsHistoryItemType.Coupon, 75);
        store.commit(SET_PAYMENTS_HISTORY, [coupon]);

        const wrapper = mount(NoPaywallInfoBar, {
            store,
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with used coupon', () => {
        const coupon: PaymentsHistoryItem = new PaymentsHistoryItem('id', 'coupon', 300, 300, 'test', '', new Date(), new Date(), PaymentsHistoryItemType.Coupon, 0);
        store.commit(SET_PAYMENTS_HISTORY, [coupon]);

        const wrapper = mount(NoPaywallInfoBar, {
            store,
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
