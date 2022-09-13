// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, mount } from '@vue/test-utils';

import SummaryItem from '@/components/project/summary/SummaryItem.vue';

const localVue = createLocalVue();

localVue.filter('centsToDollars', (cents: number): string => {
    return `$${(cents / 100).toFixed(2)}`;
});

describe('SummaryItem.vue', (): void => {
    it('renders correctly if not money', (): void => {
        const wrapper = mount(SummaryItem, {
            localVue,
            propsData: {
                backgroundColor: '#fff',
                titleColor: '#1b2533',
                valueColor: '#000',
                title: 'test',
                value: 100,
                isMoney: false,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if money', (): void => {
        const wrapper = mount(SummaryItem, {
            localVue,
            propsData: {
                backgroundColor: '#fff',
                titleColor: '#1b2533',
                valueColor: '#000',
                title: 'test',
                value: 100,
                isMoney: true,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
