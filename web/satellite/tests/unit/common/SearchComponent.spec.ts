// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount, shallowMount } from '@vue/test-utils';
import * as sinon from 'sinon';
import SearchComponent from '@/components/common/SearchComponent.vue';

describe('SearchComponent.vue', () => {
    it('renders correctly', () => {
        const wrapper = shallowMount(SearchComponent);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with default props', () => {
        const wrapper = mount(SearchComponent);

        expect(wrapper.vm.$props.placeHolder).toMatch('');
        expect(wrapper.vm.$props.search).toMatch('');
    });

    it('functions onMouseEnter/onMouseLeave work correctly', () => {
        const wrapper = mount(SearchComponent);

        wrapper.vm.onMouseEnter();

        expect(wrapper.vm.style.width).toMatch('602px');

        wrapper.vm.onMouseLeave();

        expect(wrapper.vm.searchString).toMatch('');
        expect(wrapper.vm.style.width).toMatch('56px');
    });

    it('function onInput works correctly', () => {
        let onMouseLeaveSpy = sinon.spy();
        let processSearchQuerySpy = sinon.spy();

        const wrapper = mount(SearchComponent, {
            methods: {
                onMouseLeave: onMouseLeaveSpy,
                processSearchQuery: processSearchQuerySpy,
            }
        });

        wrapper.vm.onInput();

        expect(onMouseLeaveSpy.callCount).toBe(1);
        expect(processSearchQuerySpy.callCount).toBe(1);
    });

    it('function clearSearch works correctly', () => {
        let processSearchQuerySpy = sinon.spy();

        const wrapper = mount(SearchComponent, {
            methods: {
                processSearchQuery: processSearchQuerySpy,
            }
        });

        wrapper.vm.clearSearch();

        expect(processSearchQuerySpy.callCount).toBe(1);
        expect(wrapper.vm.$data.inputWidth).toMatch('56px');
    });
});
