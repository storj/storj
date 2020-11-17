// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import SNONotification from '@/app/components/notifications/SNONotification.vue';

import { newNotificationsModule } from '@/app/store/modules/notifications';
import { UINotification } from '@/app/types/notifications';
import { NotificationsHttpApi } from '@/storagenode/api/notifications';
import { Notification, NotificationTypes } from '@/storagenode/notifications/notifications';
import { NotificationsService } from '@/storagenode/notifications/service';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

const notificationsApi = new NotificationsHttpApi();
const notificationsService = new NotificationsService(notificationsApi);
const notificationsModule = newNotificationsModule(notificationsService);

const store = new Vuex.Store({ modules: { notificationsModule }});

describe('SNONotification', (): void => {
    it('renders correctly with default props', (): void => {
        const wrapper = shallowMount(SNONotification, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly', async (): Promise<void> => {
        const mockMethod = jest.fn();
        const testNotification = new Notification(
            '123',
            '1234',
            NotificationTypes.UptimeCheckFailure,
            'title1',
            'message1',
        );

        const wrapper = shallowMount(SNONotification, {
            store,
            localVue,
            propsData: {
                notification: new UINotification(testNotification),
                isSmall: false,
            },
        });

        wrapper.setMethods({ read: mockMethod });

        await wrapper.find('.notification-item').trigger('mouseenter');

        expect(mockMethod).toHaveBeenCalled();
        expect(wrapper).toMatchSnapshot();
    });
});
