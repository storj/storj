// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import NotificationsPopup from '@/app/components/notifications/NotificationsPopup.vue';

import {
    newNotificationsModule,
    NOTIFICATIONS_MUTATIONS,
} from '@/app/store/modules/notifications';
import { NotificationsState, UINotification } from '@/app/types/notifications';
import { NotificationsHttpApi } from '@/storagenode/api/notifications';
import { Notification } from '@/storagenode/notifications/notifications';
import { NotificationsService } from '@/storagenode/notifications/service';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

const notificationsApi = new NotificationsHttpApi();
const notificationsService = new NotificationsService(notificationsApi);
const notificationsModule = newNotificationsModule(notificationsService);

const store = new Vuex.Store({ modules: { notificationsModule }});

describe('NotificationsPopup', (): void => {
    it('renders correctly with no notifications', (): void => {
        const wrapper = shallowMount(NotificationsPopup, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with less than 4 notifications', async (): Promise<void> => {
        await store.commit(NOTIFICATIONS_MUTATIONS.SET_LATEST, new NotificationsState(
            [new UINotification(new Notification())],
            1,
            1,
        ));

        const wrapper = shallowMount(NotificationsPopup, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with more than 4 notifications', async (): Promise<void> => {
        await store.commit(NOTIFICATIONS_MUTATIONS.SET_LATEST, new NotificationsState(
            [
                new UINotification(new Notification('1')),
                new UINotification(new Notification('2')),
                new UINotification(new Notification('3')),
                new UINotification(new Notification('4')),
                new UINotification(new Notification('5')),
            ],
            1,
            1,
        ));

        const wrapper = shallowMount(NotificationsPopup, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
