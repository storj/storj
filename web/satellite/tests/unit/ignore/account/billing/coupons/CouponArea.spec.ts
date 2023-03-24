// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { PaymentsMock } from '../../../../mock/api/payments';
import { FrontendConfigApiMock } from '../../../../mock/api/config';

import { makeAppStateModule } from '@/store/modules/appState';
import { makePaymentsModule } from '@/store/modules/payments';
import { Coupon, CouponDuration } from '@/types/payments';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import CouponArea from '@/components/account/billing/coupons/CouponArea.vue';

const localVue = createLocalVue();
localVue.use(Vuex);

const appStateModule = makeAppStateModule(new FrontendConfigApiMock());
const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
paymentsApi.setMockCoupon(new Coupon(
    '',
    'PROMO_CODE',
    'Coupon Name',
    123,
    0,
    new Date(2021, 9, 1),
    new Date(2021, 11, 1),
    CouponDuration.Repeating,
));

const store = new Vuex.Store({ modules: { paymentsModule, appStateModule } });
store.commit(APP_STATE_MUTATIONS.SET_COUPON_CODE_BILLING_UI_STATUS, true);

describe('CouponArea', (): void => {
    it('renders correctly', async (): Promise<void> => {
        const wrapper = shallowMount(CouponArea, {
            localVue,
            store,
        });

        await wrapper.setData({ isCouponFetching: false });

        expect(wrapper).toMatchSnapshot();
    });
});
