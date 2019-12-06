// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import * as sinon from 'sinon';

import HeaderComponent from '@/components/common/VHeader.vue';

import { mount, shallowMount } from '@vue/test-utils';

describe('HeaderComponent.vue', () => {
    it('renders correctly', () => {
        const wrapper = shallowMount(HeaderComponent);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with default props', () => {
        const wrapper = mount(HeaderComponent);

        expect(wrapper.vm.$props.placeholder).toMatch('');
        expect(wrapper.vm.$props.search).toMatch('');
    });

    it('function clearSearch works correctly', () => {
        const clearSearchSpy = sinon.spy();

        const wrapper = mount(HeaderComponent);

        wrapper.vm.$refs.search.clearSearch = clearSearchSpy;

        wrapper.vm.clearSearch();

        expect(clearSearchSpy.callCount).toBe(1);
    });
});
