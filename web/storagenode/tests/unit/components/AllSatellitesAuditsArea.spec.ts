// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { newNodeModule, NODE_ACTIONS, NODE_MUTATIONS } from '@/app/store/modules/node';
import { StorageNodeApi } from '@/storagenode/api/storagenode';
import { StorageNodeService } from '@/storagenode/sno/service';
import { Dashboard, SatelliteInfo, Satellites, SatelliteScores, Traffic } from '@/storagenode/sno/sno';

import AllSatellitesAuditsArea from '@/app/components/AllSatellitesAuditsArea.vue';

const localVue = createLocalVue();
localVue.use(Vuex);

const nodeApi = new StorageNodeApi();
const nodeService = new StorageNodeService(nodeApi);
const nodeModule = newNodeModule(nodeService);

const store = new Vuex.Store({ modules: { node: nodeModule } });

describe('AllSatellitesAuditsArea', (): void => {

    it('renders correctly with actual values', async (): Promise<void> => {
        const vettedDate = new Date(2023, 0, 15);

        // Mock dashboard data with satellites
        const dashboardInfo = new Dashboard(
            '1',
            '2',
            [],
            [
                new SatelliteInfo('sat1', 'name1', null, null, vettedDate),
                new SatelliteInfo('sat2', 'name2', null, null, null),
                new SatelliteInfo('sat3', 'name3', null, null, vettedDate),
            ],
            new Traffic(99, 100, 4),
            new Traffic(50),
            new Date(),
            new Date(),
            '0.1.1',
            '0.2.2',
            false,
            'OK',
            '13000',
            new Date(),
        );

        // Populate store with satellite info
        store.commit(NODE_MUTATIONS.POPULATE_STORE, dashboardInfo);

        const satellites = new Satellites();
        satellites.satellitesScores = [
            new SatelliteScores('name1', 1, 1, 0.5),
            new SatelliteScores('name2', 0.5, 1, 0.97),
            new SatelliteScores('name3', 0.97, 1, 1),
        ];

        jest.spyOn(nodeApi, 'satellites').mockReturnValue(Promise.resolve(satellites));

        await store.dispatch(NODE_ACTIONS.SELECT_SATELLITE);

        const wrapper = shallowMount(AllSatellitesAuditsArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
