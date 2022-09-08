// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, mount } from '@vue/test-utils';

import { RouteConfig } from '@/router';

import HistoryDropdown from '@/components/account/billing/HistoryDropdown.vue';

const localVue = createLocalVue();

describe('HistoryDropdown', (): void => {
    it('renders correctly if credit history', (): void => {
        const creditsHistory: string = RouteConfig.Account.with(RouteConfig.CreditsHistory).path;
        const wrapper = mount(HistoryDropdown, {
            localVue,
            propsData: {
                label: 'Credits History',
                route: creditsHistory,
            },
            directives: {
                clickOutside: {},
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if balance history', (): void => {
        const balanceHistory: string = RouteConfig.Account.with(RouteConfig.DepositHistory).path;
        const wrapper = mount(HistoryDropdown, {
            localVue,
            propsData: {
                label: 'Balance History',
                route: balanceHistory,
            },
            directives: {
                clickOutside: {},
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
