// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { VNode } from 'vue';
import { DirectiveBinding } from 'vue/types/options';
import Vuex from 'vuex';

import PayoutHistoryPeriodDropdown from '@/app/components/payments/PayoutHistoryPeriodDropdown.vue';

import { appStateModule } from '@/app/store/modules/appState';
import { newNodeModule, NODE_MUTATIONS } from '@/app/store/modules/node';
import { newPayoutModule, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { StorageNodeApi } from '@/storagenode/api/storagenode';
import { PayoutService } from '@/storagenode/payouts/service';
import { StorageNodeService } from '@/storagenode/sno/service';
import { Satellites } from '@/storagenode/sno/sno';
import { createLocalVue, shallowMount } from '@vue/test-utils';

let clickOutsideEvent: EventListener;

const localVue = createLocalVue();
localVue.use(Vuex);

localVue.directive('click-outside', {
    bind: function (el: HTMLElement, binding: DirectiveBinding, vnode: VNode) {
        clickOutsideEvent = function(event: Event): void {
            if (el === event.target) {
                return;
            }

            if (vnode.context && binding.expression) {
                vnode.context[binding.expression](event);
            }
        };

        document.body.addEventListener('click', clickOutsideEvent);
    },
    unbind: function(): void {
        document.body.removeEventListener('click', clickOutsideEvent);
    },
});

const payoutApi = new PayoutHttpApi();
const payoutService = new PayoutService(payoutApi);
const payoutModule = newPayoutModule(payoutService);
const nodeApi = new StorageNodeApi();
const nodeService = new StorageNodeService(nodeApi);
const nodeModule = newNodeModule(nodeService);

const store = new Vuex.Store({ modules: { payoutModule, node: nodeModule, appStateModule }});

describe('PayoutHistoryPeriodDropdown', (): void => {
    it('renders correctly with actual values', async (): Promise<void> => {
        const wrapper = shallowMount(PayoutHistoryPeriodDropdown, {
            store,
            localVue,
        });

        await store.commit(PAYOUT_MUTATIONS.SET_PAYOUT_HISTORY_PERIOD, '2020-05');

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with actual values', async (): Promise<void> => {
        const wrapper = shallowMount(PayoutHistoryPeriodDropdown, {
            store,
            localVue,
        });

        const satelliteInfo = new Satellites([], [], [], [], 0, 0, 0, 0, new Date(2020, 1));

        await store.commit(NODE_MUTATIONS.SELECT_ALL_SATELLITES, satelliteInfo);

        await wrapper.find('.period-container').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });
});
