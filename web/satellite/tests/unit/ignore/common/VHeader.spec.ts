// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount, shallowMount } from '@vue/test-utils';

import HeaderComponent from '@/components/common/VHeader.vue';

describe('HeaderComponent.vue', () => {
    it('renders correctly', () => {
        const wrapper = shallowMount(HeaderComponent);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with default props', () => {
        const wrapper = mount(HeaderComponent);
        expect(wrapper.vm.$props.placeholder).toMatch('');
    });

    it('function clearSearch works correctly', () => {
        const search = jest.fn();

        const wrapper = mount(HeaderComponent, {
            propsData: {
                search: search,
            },
        });
        wrapper.vm.clearSearch();
        expect(search).toHaveBeenCalledTimes(1);
    });
});
