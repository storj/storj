// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount } from '@vue/test-utils';

import VBar from '@/components/common/VBar.vue';

describe('VBar.vue', () => {
    it('renders correctly', () => {
        const wrapper = mount(VBar, {
            propsData: {
                current: 500,
                max: 1000,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if current > max', () => {
        const wrapper = mount(VBar, {
            propsData: {
                current: 1000,
                max: 500,
            },
        });

        expect(wrapper).toMatchSnapshot();

        const el = wrapper.find('.bar-container__fill').element as HTMLElement;
        expect(el.style.width).toMatch('100%');
    });
});
