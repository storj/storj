// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount } from '@vue/test-utils';

import TestList from '../mock/components/TestList.vue';

describe('TestList.vue', () => {
    it('should render list of primitive types', function () {
        const wrapper = mount(TestList, {
            propsData: {
                onItemClick: jest.fn(),
            },
        });
        expect(wrapper).toMatchSnapshot();
    });

    it('should retrieve callback', function () {
        const onPressSpy = jest.fn();

        const wrapper = mount(TestList, {
            propsData: {
                onItemClick: onPressSpy,
            },
        });
        wrapper.find('.item-component__item').trigger('click');
        expect(onPressSpy).toHaveBeenCalledTimes(1);
    });
});
