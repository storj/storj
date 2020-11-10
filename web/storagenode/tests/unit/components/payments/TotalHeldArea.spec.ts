// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import TotalHeldArea from '@/app/components/payments/TotalHeldArea.vue';

import { newNodeModule, NODE_MUTATIONS } from '@/app/store/modules/node';
import { newPayoutModule, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { StorageNodeApi } from '@/storagenode/api/storagenode';
import { Paystub, TotalHeldAndPaid } from '@/storagenode/payouts/payouts';
import { PayoutService } from '@/storagenode/payouts/service';
import { StorageNodeService } from '@/storagenode/sno/service';
import { Metric, Satellite, Stamp } from '@/storagenode/sno/sno';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

localVue.filter('centsToDollars', (cents: number): string => {
    return `$${(cents / 100).toFixed(2)}`;
});

const payoutApi = new PayoutHttpApi();
const payoutService = new PayoutService(payoutApi);
const payoutModule = newPayoutModule(payoutService);
const nodeApi = new StorageNodeApi();
const nodeService = new StorageNodeService(nodeApi);
const nodeModule = newNodeModule(nodeService);

const store = new Vuex.Store({ modules: { payoutModule, node: nodeModule }});

describe('TotalHeldArea', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(TotalHeldArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with actual values', async (): Promise<void> => {
        const wrapper = shallowMount(TotalHeldArea, {
            store,
            localVue,
        });

        const testJoinAt = new Date(Date.UTC(2018, 0, 30));

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
            new Metric(1, 1, 1, 0, 1),
            new Metric(2, 1, 1, 0, 1),
            testJoinAt,
        );
        const paystub = new Paystub();
        paystub.held = 600000;
        paystub.disposed = 100000;
        paystub.paid = 1000000;

        const totalHeldAndPaid = new TotalHeldAndPaid([paystub]);

        await store.commit(NODE_MUTATIONS.SELECT_SATELLITE, satelliteInfo);

        await store.commit(PAYOUT_MUTATIONS.SET_TOTAL, totalHeldAndPaid);

        expect(wrapper).toMatchSnapshot();
    });
});
