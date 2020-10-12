// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import AllSatellitesAuditsArea from '@/app/components/AllSatellitesAuditsArea.vue';

import { makeNodeModule, NODE_ACTIONS, NODE_MUTATIONS } from '@/app/store/modules/node';
import { SNOApi } from '@/storagenode/api/storagenode';
import { Satellites, SatelliteScores } from '@/storagenode/satellite';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

const nodeApi = new SNOApi();
const nodeModule = makeNodeModule(nodeApi);

const store = new Vuex.Store({ modules: { node: nodeModule }});

describe('AllSatellitesAuditsArea', (): void => {

    it('renders correctly with actual values', async (): Promise<void> => {
        const wrapper = shallowMount(AllSatellitesAuditsArea, {
            store,
            localVue,
        });

        const satellites = new Satellites();
        satellites.satellitesScores = [
            new SatelliteScores('name1', 1, 1, 0.5),
            new SatelliteScores('name2', 0.5, 1, 0.7),
            new SatelliteScores('name3', 0.7, 1, 1),
        ];

        jest.spyOn(nodeApi, 'satellites').mockReturnValue(Promise.resolve(satellites));

        await store.dispatch(NODE_ACTIONS.SELECT_SATELLITE);

        expect(wrapper).toMatchSnapshot();
    });
});
