// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount, shallowMount } from '@vue/test-utils';

import { DelayedNotification } from '@/types/DelayedNotification';
import { NOTIFICATION_TYPES } from '@/utils/constants/notification';

import NotificationArea from '@/components/notifications/NotificationArea.vue';

describe('NotificationArea.vue', () => {
    it('renders correctly', () => {
        const wrapper = shallowMount(NotificationArea, {
            computed: {
                notifications: () => [],
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with notification', () => {
        const testMessage = 'testMessage';
        const notifications = [new DelayedNotification(
            jest.fn(),
            NOTIFICATION_TYPES.SUCCESS,
            testMessage,
        ), new DelayedNotification(
            jest.fn(),
            NOTIFICATION_TYPES.ERROR,
            testMessage,
        ), new DelayedNotification(
            jest.fn(),
            NOTIFICATION_TYPES.WARNING,
            testMessage,
        ), new DelayedNotification(
            jest.fn(),
            NOTIFICATION_TYPES.NOTIFICATION,
            testMessage,
        )];

        const wrapper = mount(NotificationArea, {
            computed: {
                notifications: () => notifications,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
