// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import HeaderedInput from '@/components/common/HeaderedInput.vue';

import { mount, shallowMount } from '@vue/test-utils';

describe('HeaderedInput.vue', () => {

    it('renders correctly with default props', () => {

        const wrapper = shallowMount(HeaderedInput);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with isMultiline props', () => {

        const wrapper = shallowMount(HeaderedInput, {
            propsData: {isMultiline: true}
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.contains('textarea')).toBe(true);
        expect(wrapper.contains('input')).toBe(false);
    });

    it('renders correctly with props', () => {
        const label = 'testLabel';
        const additionalLabel = 'addLabel';
        const width = '30px';
        const height = '20px';

        const wrapper = shallowMount(HeaderedInput, {
            propsData: {label, width, height, additionalLabel}
        });

        expect(wrapper.find('input').element.style.width).toMatch(width);
        expect(wrapper.find('input').element.style.height).toMatch(height);
        expect(wrapper.find('.label-container').text()).toMatch(label);
        expect(wrapper.find('.label-container__add-label').text()).toMatch(additionalLabel);
    });

    it('renders correctly with isOptional props', () => {

        const wrapper = shallowMount(HeaderedInput, {
            propsData: {
                isOptional: true
            }
        });

        expect(wrapper.find('h4').text()).toMatch('Optional');
    });

    it('renders correctly with input error', () => {
        const error = 'testError';

        const wrapper = shallowMount(HeaderedInput, {
            propsData: {
                error
            }
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.find('.label-container').text()).toMatch(error);
    });

    it('emit setData on input correctly', () => {
        const testData = 'testData';

        const wrapper = mount(HeaderedInput);

        wrapper.find('input').trigger('input');

        expect(wrapper.emitted('setData').length).toEqual(1);

        wrapper.vm.$emit('setData', testData);

        expect(wrapper.emitted('setData')[1][0]).toEqual(testData);
    });

});
