// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import DiskStatChart from '@/app/components/DiskStatChart.vue';

import { makeNodeModule, NODE_ACTIONS } from '@/app/store/modules/node';
import { SNOApi } from '@/storagenode/api/storagenode';
import { BandwidthInfo, Dashboard, DiskSpaceInfo, SatelliteInfo } from '@/storagenode/dashboard';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

const nodeApi = new SNOApi();
const nodeModule = makeNodeModule(nodeApi);

const store = new Vuex.Store({ modules: { node: nodeModule }});

describe('DiskStatChart', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(DiskStatChart, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with actual values', async (): Promise<void> => {
        const wrapper = shallowMount(DiskStatChart, {
            store,
            localVue,
        });

        jest.spyOn(nodeApi, 'dashboard').mockReturnValue(
            Promise.resolve(
                new Dashboard(
                    '1',
                    '2',
                    [
                        new SatelliteInfo('3', 'url1', null, null),
                        new SatelliteInfo('4', 'url2', new Date(2020, 1, 1), new Date(2020, 0, 1)),
                    ],
                    new DiskSpaceInfo(550000, 1000000, 22000),
                    new BandwidthInfo(50),
                    new Date(),
                    new Date(2019, 3, 1),
                    '0.1.1',
                    '0.2.2',
                    false,
                ),
            ),
        );

        await store.dispatch(NODE_ACTIONS.GET_NODE_INFO);

        expect(wrapper).toMatchSnapshot();
    });
});
