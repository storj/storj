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
            propsData: {isMultiline: true},
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.findAll('textarea').length).toBe(1);
        expect(wrapper.findAll('input').length).toBe(0);
    });

    it('renders correctly with props', () => {
        const label = 'testLabel';
        const additionalLabel = 'addLabel';
        const width = '30px';
        const height = '20px';

        const wrapper = shallowMount(HeaderedInput, {
            propsData: {label, width, height, additionalLabel},
        });

        expect(wrapper.find('input').element.style.width).toMatch(width);
        expect(wrapper.find('input').element.style.height).toMatch(height);
        expect(wrapper.find('.label-container').text()).toMatch(label);
        expect(wrapper.find('.add-label').text()).toMatch(additionalLabel);
    });

    it('renders correctly with isOptional props', () => {

        const wrapper = shallowMount(HeaderedInput, {
            propsData: {
                isOptional: true,
            },
        });

        expect(wrapper.find('h4').text()).toMatch('Optional');
    });

    it('renders correctly with input error', () => {
        const error = 'testError';

        const wrapper = shallowMount(HeaderedInput, {
            propsData: {
                error,
            },
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.find('.label-container').text()).toMatch(error);
    });

    it('emit setData on input correctly', async () => {
        const testData = 'testData';

        const wrapper = mount(HeaderedInput);

        await wrapper.find('input').trigger('input');

        let emittedSetData = wrapper.emitted('setData');
        if (emittedSetData) expect(emittedSetData.length).toEqual(1);

        await wrapper.vm.$emit('setData', testData);

        emittedSetData = wrapper.emitted('setData');
        if (emittedSetData) expect(emittedSetData[1][0]).toEqual(testData);
    });

});
