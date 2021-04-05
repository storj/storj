// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import WalletArea from '@/app/components/WalletArea.vue';

import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();

describe('WalletArea', (): void => {

    it('renders correctly with no wallet features', (): void => {

        const wrapper = shallowMount(WalletArea, {
            localVue,
            propsData: {
                walletAddress: '0x0123456789012345678901234567890123456789',
                walletFeatures: [],
                label: 'Wallet address',
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with no wallet features', (): void => {

        const wrapper = shallowMount(WalletArea, {
            localVue,
            propsData: {
                walletAddress: '0x0123456789012345678901234567890123456789',
                walletFeatures: [ 'zksync' ],
                label: 'Wallet address',
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
