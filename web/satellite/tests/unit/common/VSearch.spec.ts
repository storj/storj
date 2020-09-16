// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import * as sinon from 'sinon';

import SearchComponent from '@/components/common/VSearch.vue';

import { mount, shallowMount } from '@vue/test-utils';

describe('SearchComponent.vue', () => {
    it('renders correctly', () => {
        const wrapper = shallowMount(SearchComponent);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with default props', () => {
        const wrapper = mount(SearchComponent);

        expect(wrapper.vm.$props.placeholder).toMatch('');
        expect(wrapper.vm.$props.search).toMatch('');
    });

    it('functions onMouseEnter/onMouseLeave work correctly', () => {
        const wrapper = mount(SearchComponent);

        wrapper.vm.onMouseEnter();

        expect(wrapper.vm.style.width).toMatch('540px');

        wrapper.vm.onMouseLeave();

        expect(wrapper.vm.searchString).toMatch('');
        expect(wrapper.vm.style.width).toMatch('56px');
    });

    it('function clearSearch works correctly', () => {
        const processSearchQuerySpy = sinon.spy();

        const wrapper = mount(SearchComponent);

        wrapper.vm.processSearchQuery = processSearchQuerySpy;
        wrapper.vm.clearSearch();

        expect(processSearchQuerySpy.callCount).toBe(1);
        expect(wrapper.vm.$data.inputWidth).toMatch('56px');
    });
});
