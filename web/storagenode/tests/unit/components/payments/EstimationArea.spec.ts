// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import EstimationArea from '@/app/components/payments/EstimationArea.vue';

import { APPSTATE_MUTATIONS, appStateModule } from '@/app/store/modules/appState';
import { newNodeModule, NODE_MUTATIONS } from '@/app/store/modules/node';
import { newPayoutModule, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
import { PayoutInfoRange } from '@/app/types/payout';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { StorageNodeApi } from '@/storagenode/api/storagenode';
import {
    EstimatedPayout,
    PayoutPeriod, Paystub,
    PreviousMonthEstimatedPayout,
    TotalPaystubForPeriod,
} from '@/storagenode/payouts/payouts';
import { PayoutService } from '@/storagenode/payouts/service';
import { StorageNodeService } from '@/storagenode/sno/service';
import { Satellites } from '@/storagenode/sno/sno';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

localVue.filter('centsToDollars', (cents: number): string => {
    return `$${(cents / 100).toFixed(2)}`;
});

const nodeApi = new StorageNodeApi();
const nodeService = new StorageNodeService(nodeApi);
const nodeModule = newNodeModule(nodeService);
const payoutApi = new PayoutHttpApi();
const payoutService = new PayoutService(payoutApi);
const payoutModule = newPayoutModule(payoutService);

const store = new Vuex.Store({ modules: { payoutModule, appStateModule, node: nodeModule }});

describe('EstimationArea', (): void => {
    it('renders correctly with actual values and current period', async (): Promise<void> => {
        const _Date = Date;
        const mockedDate1 = new Date(1580522290000); // Sat Feb 01 2020
        const mockedDate2 = new Date(1577982290000); // Thu Jan 02 2020
        const allSatellitesInfo = new Satellites();
        allSatellitesInfo.joinDate = mockedDate2;
        global.Date = jest.fn(() => mockedDate1);

        const estimatedPayout = new EstimatedPayout(
            new PreviousMonthEstimatedPayout(
                100000000,
                200000000,
                300000000,
                400000000,
                500000000,
                600000000,
                700000000,
                800000000,
                900000000,
            ),
        );

        await store.commit(PAYOUT_MUTATIONS.SET_ESTIMATION, estimatedPayout);
        await store.commit(NODE_MUTATIONS.SELECT_ALL_SATELLITES, allSatellitesInfo);
        await store.commit(PAYOUT_MUTATIONS.SET_RANGE, new PayoutInfoRange(null, new PayoutPeriod(2020, 1)));

        const wrapper = shallowMount(EstimationArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();

        global.Date = _Date;

        wrapper.destroy();
    });

    it('renders correctly with actual values and current period and not first day on month', async (): Promise<void> => {
        const _Date = Date;
        const mockedDate1 = new Date(1580722290000); // Sat Feb 03 2020
        const mockedDate2 = new Date(1577982290000); // Thu Jan 02 2020
        const allSatellitesInfo = new Satellites();
        allSatellitesInfo.joinDate = mockedDate2;
        global.Date = jest.fn(() => mockedDate1);

        const estimatedPayout = new EstimatedPayout(
            new PreviousMonthEstimatedPayout(
                10000000,
                20000000,
                30000000,
                40000000,
                50000000,
                60000000,
                70000000,
                80000000,
                90000000,
            ),
            new PreviousMonthEstimatedPayout(),
            1200000000,
        );

        await store.commit(PAYOUT_MUTATIONS.SET_ESTIMATION, estimatedPayout);
        await store.commit(NODE_MUTATIONS.SELECT_ALL_SATELLITES, allSatellitesInfo);
        await store.commit(PAYOUT_MUTATIONS.SET_RANGE, new PayoutInfoRange(null, new PayoutPeriod(2020, 1)));

        const wrapper = shallowMount(EstimationArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();

        global.Date = _Date;

        wrapper.destroy();
    });

    it('renders correctly with actual values and previous period without paystub', async (): Promise<void> => {
        const _Date = Date;
        const mockedDate1 = new Date(1580522290000); // Sat Feb 01 2020
        const mockedDate2 = new Date(1577982290000); // Thu Jan 02 2020
        const allSatellitesInfo = new Satellites();
        allSatellitesInfo.joinDate = mockedDate2;
        global.Date = jest.fn(() => mockedDate1);

        const estimatedPayout = new EstimatedPayout(
            new PreviousMonthEstimatedPayout(),
            new PreviousMonthEstimatedPayout(
                100000000,
                200000000,
                300000000,
                400000000,
                500000000,
                600000000,
                700000000,
                800000000,
                900000000,
            ),
        );

        await store.commit(PAYOUT_MUTATIONS.SET_ESTIMATION, estimatedPayout);
        await store.commit(NODE_MUTATIONS.SELECT_ALL_SATELLITES, allSatellitesInfo);
        await store.commit(PAYOUT_MUTATIONS.SET_RANGE, new PayoutInfoRange(null, new PayoutPeriod(2020, 0)));

        const wrapper = shallowMount(EstimationArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();

        global.Date = _Date;

        wrapper.destroy();
    });

    it('renders correctly with actual values and historical period with paystub', async (): Promise<void> => {
        const _Date = Date;
        const mockedDate1 = new Date(1580522290000); // Sat Feb 01 2020
        const mockedDate2 = new Date(1577982290000); // Thu Jan 02 2020
        const payoutPeriod = new PayoutPeriod(2020, 0);
        const allSatellitesInfo = new Satellites();
        allSatellitesInfo.joinDate = mockedDate2;
        global.Date = jest.fn(() => mockedDate1);

        const paystub = new Paystub();
        paystub.held = 777777;
        paystub.paid = 555555;
        paystub.surgePercent = 300;
        paystub.distributed = 333333;
        const totalPaystubForPeriod = new TotalPaystubForPeriod([paystub]);

        await store.commit(PAYOUT_MUTATIONS.SET_PERIODS, [payoutPeriod]);
        await store.commit(PAYOUT_MUTATIONS.SET_PAYOUT_INFO, totalPaystubForPeriod);
        await store.commit(NODE_MUTATIONS.SELECT_ALL_SATELLITES, allSatellitesInfo);
        await store.commit(PAYOUT_MUTATIONS.SET_RANGE, new PayoutInfoRange(null, payoutPeriod));

        const wrapper = shallowMount(EstimationArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();

        wrapper.destroy();

        global.Date = _Date;
    });

    it('renders correctly with actual values and historical period without paystub', async (): Promise<void> => {
        const _Date = Date;
        const mockedDate1 = new Date(1580522290000); // Sat Feb 01 2020
        const mockedDate2 = new Date(1577982290000); // Thu Jan 02 2020
        const payoutPeriod = new PayoutPeriod(2020, 0);
        const allSatellitesInfo = new Satellites();
        allSatellitesInfo.joinDate = mockedDate2;
        global.Date = jest.fn(() => mockedDate1);

        await store.commit(PAYOUT_MUTATIONS.SET_PERIODS, [payoutPeriod]);
        await store.commit(NODE_MUTATIONS.SELECT_ALL_SATELLITES, allSatellitesInfo);
        await store.commit(PAYOUT_MUTATIONS.SET_RANGE, new PayoutInfoRange(null, payoutPeriod));

        await store.commit(APPSTATE_MUTATIONS.SET_NO_PAYOUT_INFO, true);

        const wrapper = shallowMount(EstimationArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();

        wrapper.destroy();

        global.Date = _Date;
    });
});
