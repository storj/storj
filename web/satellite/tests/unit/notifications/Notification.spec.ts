// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount, shallowMount } from '@vue/test-utils';
import Notification from '@/components/notifications/Notification.vue';
import { NOTIFICATION_TYPES } from '@/utils/constants/notification';

describe('Notification.vue', () => {

    it('renders correctly', () => {
        const wrapper = shallowMount(Notification);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with props', () => {
        const testMessage = 'testMessage';

        const wrapper = mount(Notification, {
            propsData: {
                type: NOTIFICATION_TYPES.SUCCESS,
                message: testMessage,
            },
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.find('.notification-wrap__text').text()).toMatch(testMessage);

        wrapper.setProps({
            type: NOTIFICATION_TYPES.ERROR,
            message: testMessage,
        });

        expect(wrapper).toMatchSnapshot();

        wrapper.setProps({
            type: NOTIFICATION_TYPES.NOTIFICATION,
            message: testMessage,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
