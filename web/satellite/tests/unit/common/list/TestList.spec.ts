// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount, shallowMount } from '@vue/test-utils';
import TestList from '@/components/common/test/TestList.vue';
import * as sinon from 'sinon';

describe('TestList.vue', () => {
    it('should render list of primitive types', function () {
        const wrapper = mount(TestList);
        expect(wrapper.html()).toBe( '<div class="item-component"><h1 class="item-component__item">1</h1><h1 class="item-component__item">2</h1><h1 class="item-component__item">3</h1></div>');
    });

    it('should retrieve callback', function () {
        let onPressSpy = sinon.spy();

        const w= mount(TestList, {
            propsData: {
                onItemClick: onPressSpy,
            }
        });
        w.find('.item-component__item').trigger('click');
        expect(onPressSpy.callCount).toBe(1);
    });


});
