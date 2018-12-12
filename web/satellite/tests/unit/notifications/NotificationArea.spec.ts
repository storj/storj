// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import { shallowMount, mount } from '@vue/test-utils';
import NotificationArea from '@/components/notifications/NotificationArea.vue';
import { NOTIFICATION_TYPES } from '@/utils/constants/notification';
import { DelayedNotification } from '@/utils/entities/DelayedNotification';
import Vuex from 'vuex';
import { createLocalVue } from 'vue-test-utils';


const localVue = createLocalVue();
localVue.use(Vuex);

describe('Notification.vue', () => {
	
	it('renders correctly', () => {
    	const wrapper = shallowMount(NotificationArea,{
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
            localVue,
            computed: {
                currentNotification: () => notification,
            }
        });

        expect(wrapper).toMatchSnapshot();
    });
});
