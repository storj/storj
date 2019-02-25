// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount, shallowMount } from '@vue/test-utils';
import AccountDropdown from '@/components/header/AccountDropdown.vue';
import * as sinon from 'sinon';

describe('AccountDropdown.vue', () => {

    it('renders correctly', () => {

        const wrapper = shallowMount(AccountDropdown);

        expect(wrapper).toMatchSnapshot();
    });

    it('trigger onPress correctly', () => {
        let onCloseSpy = sinon.spy();

        const wrapper = mount(AccountDropdown, {
            propsData: {
                onClose: onCloseSpy
            },
            mocks: {
                $router: {
                    push: onCloseSpy
                }
            }
        });

        wrapper.find('.adItemContainer.settings').trigger('click');

        expect(onCloseSpy.callCount).toBe(1);
        expect(wrapper.emitted('onClose').length).toEqual(1);
    });
});
