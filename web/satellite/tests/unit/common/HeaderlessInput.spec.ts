// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount, shallowMount } from '@vue/test-utils';
import HeaderlessInput from '@/components/common/HeaderlessInput.vue';

describe('HeaderlessInput.vue', () => {

    it('renders correctly with default props', () => {

        const wrapper = shallowMount(HeaderlessInput);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with size props', () => {
        let placeholder = 'test';
        let width = '30px';
        let height = '20px';

        const wrapper = shallowMount(HeaderlessInput, {
            propsData: {placeholder, width, height}
        });

        expect(wrapper.find('input').element.style.width).toMatch(width);
        expect(wrapper.find('input').element.style.height).toMatch(height);
        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with isPassword prop', () => {
        const wrapper = mount(HeaderlessInput, {
            propsData: {isPassword: true}
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('emit setData on input correctly', () => {
        let testData = 'testData';

        const wrapper = mount(HeaderlessInput);

        wrapper.find('input').trigger('input');

        expect(wrapper.emitted('setData').length).toEqual(1);

        wrapper.vm.$emit('setData', testData);

        expect(wrapper.emitted('setData')[1][0]).toEqual(testData);
    });

});
