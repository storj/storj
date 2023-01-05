// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount, shallowMount } from '@vue/test-utils';

import VButton from '@/components/common/VButton.vue';

describe('Button.vue', () => {
    it('renders correctly', () => {
        // VButton as never is not ideal. We will not need if after we upgrade from @vue/test-utils@1.3.0
        const wrapper = mount(VButton as never);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with isWhite prop', () => {
        const wrapper = shallowMount(VButton as never, {
            propsData: { isWhite: true },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with isDisabled prop', () => {
        const wrapper = shallowMount(VButton as never, {
            propsData: { isDisabled: true },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with size and label props', () => {
        const label = 'testLabel';
        const width = '30px';
        const height = '20px';

        const wrapper = shallowMount(VButton as never, {
            propsData: {
                label,
                width,
                height,
            },
        });

        const el = wrapper.element as HTMLElement;
        expect(el.style.width).toMatch(width);
        expect(el.style.height).toMatch(height);
        expect(wrapper.text()).toMatch(label);
    });

    it('renders correctly with default props', () => {
        const wrapper = shallowMount(VButton as never);

        const el = wrapper.element as HTMLElement;
        expect(el.style.width).toMatch('inherit');
        expect(el.style.height).toMatch('inherit');
        expect(wrapper.text()).toMatch('Default');
    });

    it('trigger onPress correctly', () => {
        const onPressSpy = jest.fn();
        const onPress = onPressSpy;
        const isDisabled = false;

        const wrapper = shallowMount(VButton as never, {
            propsData: {
                onPress,
                isDisabled,
            },
        });

        wrapper.find('div.container').trigger('click');

        expect(onPressSpy).toHaveBeenCalledTimes(1);
    });
});
