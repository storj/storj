// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import PayoutPeriodCalendar from '@/app/components/payments/PayoutPeriodCalendar.vue';

import { appStateModule } from '@/app/store/modules/appState';
import { newNodeModule, NODE_MUTATIONS } from '@/app/store/modules/node';
import { newPayoutModule, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { StorageNodeApi } from '@/storagenode/api/storagenode';
import { PayoutPeriod } from '@/storagenode/payouts/payouts';
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
const _Date: DateConstructor = Date;
let wrapper;

describe('PayoutPeriodCalendar', (): void => {
    beforeEach(async () => {
        const mockedDate1 = new Date(Date.UTC(2020, 1, 30));
        const mockedDate2 = new Date(1573562290000); // Tue Nov 12 2019
        const allSatellitesInfo = new Satellites();
        allSatellitesInfo.joinDate = mockedDate2;
        const payoutPeriod1 = new PayoutPeriod(2019, 11);
        const payoutPeriod2 = new PayoutPeriod(2020, 0);
        global.Date = jest.fn(() => mockedDate1);

        await store.commit(PAYOUT_MUTATIONS.SET_PERIODS, [payoutPeriod1, payoutPeriod2]);
        await store.commit(NODE_MUTATIONS.SELECT_ALL_SATELLITES, allSatellitesInfo);

        wrapper = shallowMount(PayoutPeriodCalendar, {
            store,
            localVue,
        });
    });
    afterEach(() => {
        global.Date = _Date;

        wrapper.destroy();
    });

    it('renders correctly', async (): Promise<void> => {
        wrapper.vm.$mount();

        await expect(wrapper).toMatchSnapshot();
        expect(wrapper.vm.displayedYear).toBe(2020);

        await wrapper.find('.payout-period-calendar__header__year-selection__next').trigger('click');

        expect(wrapper.vm.displayedYear).toBe(2020);

        await wrapper.find('.payout-period-calendar__header__year-selection__prev').trigger('click');

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.vm.displayedYear).toBe(2019);
    });

    it('selects month correctly', async (): Promise<void> => {
        wrapper.vm.$mount();

        await wrapper.findAll('.month-item').at(3).trigger('click');

        expect(wrapper.vm.firstSelectedMonth).toBe(null);
        expect(wrapper.vm.secondSelectedMonth).toBe(null);

        await wrapper.findAll('.month-item').at(0).trigger('click');
        expect(wrapper.vm.firstSelectedMonth.year).toBe(2020);
        expect(wrapper.vm.firstSelectedMonth.index).toBe(0);
        expect(wrapper.vm.secondSelectedMonth).toBe(null);

        await wrapper.findAll('.month-item').at(0).trigger('click');
        expect(wrapper.vm.firstSelectedMonth).toBe(null);
        expect(wrapper.vm.secondSelectedMonth).toBe(null);
    });

    it('selects months range correctly', async (): Promise<void> => {
        wrapper.vm.$mount();

        await wrapper.findAll('.month-item').at(0).trigger('click');
        expect(wrapper.vm.firstSelectedMonth.year).toBe(2020);
        expect(wrapper.vm.firstSelectedMonth.index).toBe(0);
        expect(wrapper.vm.secondSelectedMonth).toBe(null);

        await wrapper.find('.payout-period-calendar__header__year-selection__prev').trigger('click');

        await wrapper.findAll('.month-item').at(11).trigger('click');
        expect(wrapper.vm.firstSelectedMonth.year).toBe(2020);
        expect(wrapper.vm.firstSelectedMonth.index).toBe(0);
        expect(wrapper.vm.secondSelectedMonth.year).toBe(2019);
        expect(wrapper.vm.secondSelectedMonth.index).toBe(11);
    });

    it('selects all time correctly', async (): Promise<void> => {
        wrapper.vm.$mount();

        await wrapper.find('.payout-period-calendar__header__all-time').trigger('click');

        expect(wrapper.vm.firstSelectedMonth.year).toBe(2019);
        expect(wrapper.vm.firstSelectedMonth.index).toBe(10);
        expect(wrapper.vm.secondSelectedMonth.year).toBe(2020);
        expect(wrapper.vm.secondSelectedMonth.index).toBe(0);
    });
});
