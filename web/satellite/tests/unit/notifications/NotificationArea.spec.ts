// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { shallowMount, mount } from '@vue/test-utils';
import NotificationArea from '@/components/notifications/NotificationArea.vue';
import { NOTIFICATION_TYPES } from '@/utils/constants/notification';
import { DelayedNotification } from '@/types/DelayedNotification';

describe('Notification.vue', () => {

    it('renders correctly', () => {
        const wrapper = shallowMount(NotificationArea, {
            computed: {
                currentNotification: jest.fn(),
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with notification', () => {
        const testMessage = 'testMessage';
        const notification = new DelayedNotification(
            jest.fn(),
            NOTIFICATION_TYPES.SUCCESS,
            testMessage
        );

        const wrapper = mount(NotificationArea, {
            computed: {
                currentNotification: () => notification,
            }
        });

        expect(wrapper).toMatchSnapshot();
    });
});
