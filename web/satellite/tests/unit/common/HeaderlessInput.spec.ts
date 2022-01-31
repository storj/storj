// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';

import { mount, shallowMount } from '@vue/test-utils';

describe('HeaderlessInput.vue', () => {
    it('renders correctly with default props', () => {

        const wrapper = shallowMount(HeaderlessInput);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with size props', () => {
        const placeholder = 'test';
        const width = '30px';
        const height = '20px';

        const wrapper = shallowMount(HeaderlessInput, {
            propsData: {placeholder, width, height},
        });

        expect(wrapper.find('input').element.style.width).toMatch(width);
        expect(wrapper.find('input').element.style.height).toMatch(height);
        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with isPassword prop', () => {
        const wrapper = mount(HeaderlessInput, {
            propsData: {isPassword: true},
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('emit setData on input correctly', () => {
        const testData = 'testData';

        const wrapper = mount(HeaderlessInput);

        wrapper.find('input').trigger('input');

        let emittedSetData = wrapper.emitted('setData');
        if (emittedSetData) expect(emittedSetData.length).toEqual(1);

        wrapper.vm.$emit('setData', testData);

        emittedSetData = wrapper.emitted('setData');
        if (emittedSetData) expect(emittedSetData[1][0]).toEqual(testData);
    });

});
