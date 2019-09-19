// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';

import TestList from '@/components/common/test/TestList.vue';

import { mount } from '@vue/test-utils';

describe('TestList.vue', () => {
    it('should render list of primitive types', function () {
        const wrapper = mount(TestList, {
            propsData: {
                onItemClick: sinon.stub(),
            },
        });
        expect(wrapper).toMatchSnapshot();
    });

    it('should retrieve callback', function () {
        const onPressSpy = sinon.spy();

        const wrapper = mount(TestList, {
            propsData: {
                onItemClick: onPressSpy,
            },
        });
        wrapper.find('.item-component__item').trigger('click');
        expect(onPressSpy.callCount).toBe(1);
    });
});
