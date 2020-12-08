// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import DiskStatChart from '@/app/components/DiskStatChart.vue';

import { newNodeModule, NODE_ACTIONS } from '@/app/store/modules/node';
import { StorageNodeApi } from '@/storagenode/api/storagenode';
import { StorageNodeService } from '@/storagenode/sno/service';
import { Dashboard, SatelliteInfo, Traffic } from '@/storagenode/sno/sno';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

const nodeApi = new StorageNodeApi();
const nodeService = new StorageNodeService(nodeApi);
const nodeModule = newNodeModule(nodeService);

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
                    new Traffic(550000, 1000000, 22000),
                    new Traffic(50),
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
