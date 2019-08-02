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
        const wrapper = shallowMount(SearchComponent);

        expect(wrapper.find('input').props('placeHolder')).toMatch('');
        expect(wrapper.find('input').props('search')).toMatch('');
    });

    it('functions onMouseEnter/onMouseLeave work correctly', () => {
        const wrapper = mount(SearchComponent);

        wrapper.vm.onMouseEnter();

        expect(wrapper.vm.inputWidth).toMatch('602px');

        wrapper.vm.onMouseLeave();

        expect(wrapper.vm.searchQuery).toMatch('');
        expect(wrapper.vm.inputWidth).toMatch('56px');
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

    it('function processSearchQuery works correctly', async () => {
        const wrapper = mount(SearchComponent);

        wrapper.vm.processSearchQuery();

        expect(wrapper.vm.search).resolves;
    });
});
