// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import EstimationPeriodDropdown from '@/app/components/payments/EstimationPeriodDropdown.vue';

import { appStateModule } from '@/app/store/modules/appState';
import { makeNodeModule, NODE_MUTATIONS } from '@/app/store/modules/node';
import { SNOApi } from '@/storagenode/api/storagenode';
import { BandwidthInfo, Dashboard, DiskSpaceInfo, SatelliteInfo } from '@/storagenode/dashboard';
import { Metric, Satellite, Stamp } from '@/storagenode/satellite';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const nodeApi = new SNOApi();
const nodeModule = makeNodeModule(nodeApi);

const localVue = createLocalVue();
localVue.use(Vuex);

Vue.directive('click-outside', {
    bind: (): void => { return; },
    unbind: (): void => { return; },
});

const store = new Vuex.Store({ modules: { appStateModule, node: nodeModule }});

describe('DiskStatChart', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(EstimationPeriodDropdown, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('opens calendar on click only when historical data exists',   async (): Promise<void> => {
        const satelliteInfo = new Satellite(
            '3',
            [new Stamp()],
            [],
            [],
            [],
            111,
            222,
            50,
            70,
            new Metric(1, 1, 1, 0, 1, 0, 0, 1),
            new Metric(2, 1, 1, 0, 1, 0, 0, 1),
            new Date(),
        );

        const dashboardInfo = new Dashboard(
            '1',
            '2',
            [
                new SatelliteInfo('3', 'url1', null, null),
                new SatelliteInfo('4', 'url2', new Date(), new Date(2020, 0, 1)),
            ],
            new DiskSpaceInfo(99, 100, 4),
            new BandwidthInfo(50),
            new Date(),
            new Date(),
            '0.1.1',
            '0.2.2',
            false,
        );

        store.commit(NODE_MUTATIONS.POPULATE_STORE, dashboardInfo);
        store.commit(NODE_MUTATIONS.SELECT_SATELLITE, satelliteInfo);

        const wrapper = shallowMount(EstimationPeriodDropdown, {
            store,
            localVue,
        });

        wrapper.find('.period-container').trigger('click');

        await localVue.nextTick();
        expect(wrapper.find('.period-container__calendar').exists()).toBe(false);

        satelliteInfo.joinDate = new Date(Date.UTC(2019, 10, 29));

        store.commit(NODE_MUTATIONS.SELECT_SATELLITE, satelliteInfo);

        wrapper.find('.period-container').trigger('click');

        await localVue.nextTick();
        expect(wrapper.find('.period-container__calendar').exists()).toBe(true);
    });
});
