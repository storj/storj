// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import SNONotification from '@/app/components/notifications/SNONotification.vue';

import { newNotificationsModule } from '@/app/store/modules/notifications';
import { UINotification } from '@/app/types/notifications';
import { NotificationsHttpApi } from '@/storagenode/api/notifications';
import { Notification, NotificationTypes } from '@/storagenode/notifications/notifications';
import { NotificationsService } from '@/storagenode/notifications/service';
import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

const notificationsApi = new NotificationsHttpApi();
const notificationsService = new NotificationsService(notificationsApi);
const notificationsModule = newNotificationsModule(notificationsService);

const store = new Vuex.Store({ modules: { notificationsModule }});

describe('SNONotification', (): void => {
    it('renders correctly with default props', (): void => {
        const wrapper = mount(SNONotification, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly', async (): Promise<void> => {
        const note = new Notification(
            '123',
            '1234',
            NotificationTypes.AuditCheckFailure,
            'title1',
            'message1',
        );
        const uinote = new UINotification(note);

        const wrapper = mount(SNONotification, {
            store,
            localVue,
            propsData: {
                notification: uinote,
                isSmall: false,
            },
        });

        await wrapper.find('.notification-item').trigger('mouseenter');

        // TODO: verify that the notification has been marked as read
        //   expect(state.notifications[0].isRead).toBe(true);
        expect(wrapper).toMatchSnapshot();
    });
});
