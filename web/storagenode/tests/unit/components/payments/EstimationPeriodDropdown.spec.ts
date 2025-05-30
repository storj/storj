// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { appStateModule } from '@/app/store/modules/appState';
import { newNodeModule, NODE_MUTATIONS, QUIC_STATUS } from '@/app/store/modules/node';
import { StorageNodeApi } from '@/storagenode/api/storagenode';
import { StorageNodeService } from '@/storagenode/sno/service';
import {
    Dashboard,
    Satellite,
    SatelliteInfo, SatelliteScores,
    Stamp,
    Traffic,
} from '@/storagenode/sno/sno';

import EstimationPeriodDropdown from '@/app/components/payments/EstimationPeriodDropdown.vue';

const nodeApi = new StorageNodeApi();
const nodeService = new StorageNodeService(nodeApi);
const nodeModule = newNodeModule(nodeService);

const localVue = createLocalVue();
localVue.use(Vuex);

Vue.directive('click-outside', {
    bind: (): void => { return; },
    unbind: (): void => { return; },
});

const store = new Vuex.Store({ modules: { appStateModule, node: nodeModule } });

describe('EstimationPeriodDropdown', (): void => {
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
            11,
            222,
            50,
            70,
            new SatelliteScores('', 1, 0, 0),
            new Date(),
        );

        const dashboardInfo = new Dashboard(
            '1',
            '2',
            [],
            [
                new SatelliteInfo('3', 'url1', null, null, null),
                new SatelliteInfo('4', 'url2', new Date(), new Date(2020, 0, 1), null),
            ],
            new Traffic(99, 100, 4),
            new Traffic(50),
            new Date(),
            new Date(),
            '0.1.1',
            '0.2.2',
            false,
            QUIC_STATUS.StatusOk,
            '13000',
            new Date(),
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
