// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount, shallowMount } from '@vue/test-utils';
import Button from '@/components/common/Button.vue';
import * as sinon from 'sinon';

describe('Button.vue', () => {

    it('renders correctly', () => {

        const wrapper = shallowMount(Button);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with isWhite prop', () => {

        const wrapper = shallowMount(Button, {
            propsData: {
                isWhite: true
            }
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with isDisabled prop', () => {

        const wrapper = shallowMount(Button, {
            propsData: {
                isDisabled: true
            }
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with size and label props', () => {
        let label = 'testLabel';
        let width = '30px';
        let height = '20px';

        const wrapper = shallowMount(Button, {
            propsData: {label, width, height},
        });

        expect(wrapper.element.style.width).toMatch(width);
        expect(wrapper.element.style.height).toMatch(height);
        expect(wrapper.text()).toMatch(label);
    });

    it('renders correctly with default props', () => {

        const wrapper = shallowMount(Button);

        expect(wrapper.element.style.width).toMatch('inherit');
        expect(wrapper.element.style.height).toMatch('inherit');
        expect(wrapper.text()).toMatch('Default');
    });

    it('trigger onPress correctly', () => {
        let onPressSpy = sinon.spy();

        const wrapper = mount(Button, {
            propsData: {
                onPress: onPressSpy,
                isDisabled: false
            }
        });

        wrapper.find('div.container').trigger('click');

        expect(onPressSpy.callCount).toBe(1);
    });
});
