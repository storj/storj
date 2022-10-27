// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { shallowMount } from '@vue/test-utils';

import VInput from '@/components/common/VInput.vue';

describe('VInput.vue', () => {
    it('renders correctly with default props', () => {

        const wrapper = shallowMount(VInput as never);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with isMultiline props', () => {
        const wrapper = shallowMount(VInput as never, {
            propsData: {
                isMultiline: true,
            },
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

        const wrapper = shallowMount(VInput as never, {
            propsData : {
                label,
                additionalLabel,
                width,
                height,
            },
        });

        const el = wrapper.find('input').element as HTMLElement;
        expect(el.style.width).toMatch(width);
        expect(el.style.height).toMatch(height);

        expect(wrapper.find('.label-container').text()).toMatch(label);
        expect(wrapper.find('.add-label').text()).toMatch(additionalLabel);
    });

    it('renders correctly with isOptional props', () => {
        const wrapper = shallowMount(VInput as never, {
            propsData: {
                isOptional: true,
            },
        });

        expect(wrapper.find('h4').text()).toMatch('Optional');
    });

    it('renders correctly with input error', () => {
        const error = 'testError';

        const wrapper = shallowMount(VInput as never, {
            propsData: {
                error,
            },
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.find('.label-container').text()).toMatch(error);
    });

    it('emit setData on input correctly', async () => {
        const testData = 'testData';

        const wrapper = shallowMount(VInput as never);

        await wrapper.find('input').trigger('input');

        let emittedSetData = wrapper.emitted('setData');
        if (emittedSetData) expect(emittedSetData.length).toEqual(1);

        await wrapper.vm.$emit('setData', testData);

        emittedSetData = wrapper.emitted('setData');
        if (emittedSetData) expect(emittedSetData[1][0]).toEqual(testData);
    });

});
