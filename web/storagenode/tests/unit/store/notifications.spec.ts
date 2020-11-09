// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import { newNotificationsModule, NOTIFICATIONS_ACTIONS, NOTIFICATIONS_MUTATIONS } from '@/app/store/modules/notifications';
import { NotificationsState, UINotification } from '@/app/types/notifications';
import { NotificationsHttpApi } from '@/storagenode/api/notifications';
import {
    Notification,
    NotificationsPage,
    NotificationsResponse,
    NotificationTypes,
} from '@/storagenode/notifications/notifications';
import { NotificationsService } from '@/storagenode/notifications/service';
import { createLocalVue } from '@vue/test-utils';

const Vue = createLocalVue();

const notificationsApi = new NotificationsHttpApi();
const notificationsService = new NotificationsService(notificationsApi);

const notificationsModule = newNotificationsModule(notificationsService);

Vue.use(Vuex);

const store = new Vuex.Store({ modules: { notificationsModule } });

const state = store.state as any;

let notifications;

describe('mutations', () => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
        notifications = [
            new UINotification(new Notification('1', '1', NotificationTypes.Disqualification, 'title1', 'message1', null)),
            new UINotification(new Notification('2', '1', NotificationTypes.UptimeCheckFailure, 'title2', 'message2', null)),
        ];
    });

    it('sets notification state', (): void => {
        const notificationsState = new NotificationsState(notifications, 2, 1);

        store.commit(NOTIFICATIONS_MUTATIONS.SET_NOTIFICATIONS, notificationsState);

        expect(state.notificationsModule.notifications.length).toBe(notifications.length);
        expect(state.notificationsModule.pageCount).toBe(2);
        expect(state.notificationsModule.unreadCount).toBe(1);

        store.commit(NOTIFICATIONS_MUTATIONS.SET_LATEST, notificationsState);

        expect(state.notificationsModule.latestNotifications.length).toBe(notifications.length);
    });

    it('sets single notification as read', (): void => {
        store.commit(NOTIFICATIONS_MUTATIONS.MARK_AS_READ, '1');

        const unreadNotificationsCount = state.notificationsModule.notifications.filter(e => !e.isRead).length;

        expect(unreadNotificationsCount).toBe(1);
    });

    it('sets all notification as read', (): void => {
        store.commit(NOTIFICATIONS_MUTATIONS.READ_ALL);

        const unreadNotificationsCount = state.notificationsModule.notifications.filter(e => !e.isRead).length;

        expect(unreadNotificationsCount).toBe(0);
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
        notifications = [
            new UINotification(new Notification('1', '1', NotificationTypes.Disqualification, 'title1', 'message1', null)),
            new UINotification(new Notification('2', '1', NotificationTypes.UptimeCheckFailure, 'title2', 'message2', null)),
        ];
    });

    it('throws error on failed notifications fetch', async (): Promise<void> => {
        jest.spyOn(notificationsApi, 'get').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(NOTIFICATIONS_ACTIONS.GET_NOTIFICATIONS, 1);
            expect(true).toBe(false);
        } catch (error) {
            expect(state.notificationsModule.latestNotifications.length).toBe(notifications.length);
        }
    });

    it('success fetches notifications', async (): Promise<void> => {
        jest.spyOn(notificationsService, 'notifications')
            .mockReturnValue(Promise.resolve(new NotificationsResponse(new NotificationsPage(notifications, 1), 2, 1)));

        await store.dispatch(NOTIFICATIONS_ACTIONS.GET_NOTIFICATIONS, 1);

        expect(state.notificationsModule.latestNotifications.length).toBe(notifications.length);
    });

    it('throws error on failed single notification read', async (): Promise<void> => {
        jest.spyOn(notificationsApi, 'read').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(NOTIFICATIONS_ACTIONS.MARK_AS_READ, '1');
            expect(true).toBe(false);
        } catch (error) {
            const unreadNotificationsCount = state.notificationsModule.notifications.filter(e => !e.isRead).length;

            expect(unreadNotificationsCount).toBe(notifications.length);
        }
    });

    it('success marks single notification as read', async (): Promise<void> => {
        jest.spyOn(notificationsService, 'notifications')
            .mockReturnValue(Promise.resolve(new NotificationsResponse(new NotificationsPage(notifications, 1), 2, 1)));

        await store.dispatch(NOTIFICATIONS_ACTIONS.GET_NOTIFICATIONS, 1);
        await store.dispatch(NOTIFICATIONS_ACTIONS.MARK_AS_READ, '1');

        const unreadNotificationsCount = state.notificationsModule.notifications.filter(e => !e.isRead).length;

        expect(unreadNotificationsCount).toBe(1);
    });

    it('throws error on failed all notifications read', async (): Promise<void> => {
        jest.spyOn(notificationsApi, 'readAll').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(NOTIFICATIONS_ACTIONS.READ_ALL);
            expect(true).toBe(false);
        } catch (error) {
            const unreadNotificationsCount = state.notificationsModule.notifications.filter(e => !e.isRead).length;

            expect(unreadNotificationsCount).toBe(1);
        }
    });

    it('success marks all notifications as read', async (): Promise<void> => {
        await store.dispatch(NOTIFICATIONS_ACTIONS.READ_ALL);

        const unreadNotificationsCount = state.notificationsModule.notifications.filter(e => !e.isRead).length;

        expect(unreadNotificationsCount).toBe(0);
    });
});
