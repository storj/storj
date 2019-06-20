// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';
import { ACTIONS, MUTATIONS } from '@/utils/constants';

Vue.use(Vuex);

// storage node store (vuex)
const store = new Vuex.Store({
    state: {
        node: {
            id: '12QKex7UUaFeX728x6divdRUApCsm2QybxTdvuWbG1SRdmJqfd1',
            status: 'Online',
            version: 'v0.11.1',
            wallet: '0xb64ef51c888972c908cfacf59b47c1afbc0ab8ac',
        },

        satellite: {
            list: [
                {
                    name: 'US-East-1',
                    id: 0,
                    isSelected: false,
                },
                {
                    name: 'Two',
                    id: 1,
                    isSelected: false,
                },
                {
                    name: 'Three',
                    id: 2,
                    isSelected: false,
                },
                {
                    name: 'Four',
                    id: 3,
                    isSelected: false,
                },
                {
                    name: 'Five',
                    id: 4,
                    isSelected: false,
                },
            ],
            selected: 'US-East-1',
        },

        wallet: {
            address: '0xb64ef51c888972c908cfacf59b47c1afbc0ab8ac',
        },

        bandwidth: {
            used: '165',
            remaining: '234',
        },

        diskSpace: {
            used: '82',
            remaining: '544',
        },

        checks: {
            uptime: '93.7',
            audit: '23.5',
        },
    },

    mutations: {

    },
    actions: {

    },
});

export default store;
