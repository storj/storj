// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount, shallowMount } from '@vue/test-utils';
import Checkbox from '@/components/common/Checkbox.vue';

describe('Checkbox.vue', () => {

    it('renders correctly', () => {

        const wrapper = shallowMount(Checkbox);

        expect(wrapper).toMatchSnapshot();
    });

    it('emit setData on change correctly', () => {

        const wrapper = mount(Checkbox);

        wrapper.find('input').trigger('change');
        wrapper.find('input').trigger('change');

        expect(wrapper.emitted('setData').length).toEqual(2);
    });

    it('emits with data correctly', () => {

        const wrapper = mount(Checkbox);

        wrapper.vm.$emit('setData', true);

        expect(wrapper.emitted('setData')[0][0]).toEqual(true);
    });

    it('renders correctly with error', () => {

        const wrapper = shallowMount(Checkbox, {
            propsData: {isCheckboxError: true}
        });

        expect(wrapper).toMatchSnapshot();
    });
});
