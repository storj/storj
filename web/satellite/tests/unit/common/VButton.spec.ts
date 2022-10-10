// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount, shallowMount } from '@vue/test-utils';

import Button from '@/components/common/VButton.vue';

describe('Button.vue', () => {
    it('renders correctly', () => {
        const wrapper = shallowMount(Button, {
            propsData: {
                onPress: () => { return; },
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with isWhite prop', () => {
        const wrapper = shallowMount(Button, {
            propsData: {
                isWhite: true,
                onPress: () => { return; },
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with isDisabled prop', () => {
        const wrapper = shallowMount(Button, {
            propsData: {
                isDisabled: true,
                onPress: () => { return; },
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with size and label props', () => {
        const label = 'testLabel';
        const width = '30px';
        const height = '20px';

        const wrapper = shallowMount(Button, {
            propsData: {
                label,
                width,
                height,
                onPress: () => { return; },
            },
        });

        const el = wrapper.element as HTMLElement;
        expect(el.style.width).toMatch(width);
        expect(el.style.height).toMatch(height);
        expect(wrapper.text()).toMatch(label);
    });

    it('renders correctly with default props', () => {
        const wrapper = shallowMount(Button, {
            propsData: {
                onPress: () => { return; },
            },
        });

        const el = wrapper.element as HTMLElement;
        expect(el.style.width).toMatch('inherit');
        expect(el.style.height).toMatch('inherit');
        expect(wrapper.text()).toMatch('Default');
    });

    it('trigger onPress correctly', () => {
        const onPressSpy = jest.fn();

        const wrapper = mount(Button, {
            propsData: {
                onPress: onPressSpy,
                isDisabled: false,
            },
        });

        wrapper.find('div.container').trigger('click');

        expect(onPressSpy).toHaveBeenCalledTimes(1);
    });
});
