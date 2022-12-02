// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, mount } from '@vue/test-utils';

import { makeNotificationsModule } from '@/store/modules/notifications';
import { DelayedNotification } from '@/types/DelayedNotification';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
import { NOTIFICATION_TYPES } from '@/utils/constants/notification';

import NotificationItem from '@/components/notifications/NotificationItem.vue';

const localVue = createLocalVue();

localVue.use(Vuex);
const pauseSpy = jest.fn();
const resumeSpy = jest.fn();
const deleteSpy = jest.fn();
const notificationModule = makeNotificationsModule();
notificationModule.actions[NOTIFICATION_ACTIONS.PAUSE] = pauseSpy;
notificationModule.actions[NOTIFICATION_ACTIONS.RESUME] = resumeSpy;
notificationModule.actions[NOTIFICATION_ACTIONS.DELETE] = deleteSpy;

const store = new Vuex.Store(notificationModule);

describe('NotificationItem', () => {

    it('renders correctly', () => {
        const wrapper = mount(NotificationItem);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with success props', () => {
        const testMessage = 'testMessage';

        const wrapper = mount(NotificationItem, {
            propsData: {
                notification: new DelayedNotification(
                    jest.fn(),
                    NOTIFICATION_TYPES.SUCCESS,
                    testMessage,
                ),
            },
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.find('.notification-wrap__content-area__message').text()).toMatch(testMessage);
    });

    it('renders correctly with error props', () => {
        const testMessage = 'testMessage';

        const wrapper = mount(NotificationItem, {
            propsData: {
                notification: new DelayedNotification(
                    jest.fn(),
                    NOTIFICATION_TYPES.ERROR,
                    testMessage,
                ),
            },
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.find('.notification-wrap__content-area__message').text()).toMatch(testMessage);
    });

    it('renders correctly with notification props', () => {
        const testMessage = 'testMessage';

        const wrapper = mount(NotificationItem, {
            propsData: {
                notification: new DelayedNotification(
                    jest.fn(),
                    NOTIFICATION_TYPES.NOTIFICATION,
                    testMessage,
                ),
            },
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.find('.notification-wrap__content-area__message').text()).toMatch(testMessage);
    });

    it('renders correctly with warning props', () => {
        const testMessage = 'testMessage';

        const wrapper = mount(NotificationItem, {
            propsData: {
                notification: new DelayedNotification(
                    jest.fn(),
                    NOTIFICATION_TYPES.WARNING,
                    testMessage,
                ),
            },
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.find('.notification-wrap__content-area__message').text()).toMatch(testMessage);
    });

    it('renders correctly with support link', async (): Promise<void> => {
        const testMessage = 'testMessage support rest of message';

        const wrapper = mount(NotificationItem, {
            propsData: {
                notification: new DelayedNotification(
                    jest.fn(),
                    NOTIFICATION_TYPES.NOTIFICATION,
                    testMessage,
                ),
            },
        });

        await wrapper.setData({ requestUrl: 'https://requestUrl.com' });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.find('.notification-wrap__content-area__message').text()).toMatch(testMessage);
        expect(wrapper.find('.notification-wrap__content-area__link').text()).toBeTruthy();
    });

    it('trigger pause correctly', () => {
        const wrapper = mount(NotificationItem, { store, localVue });

        wrapper.find('.notification-wrap').trigger('mouseover');

        expect(pauseSpy).toHaveBeenCalledTimes(1);
    });

    it('trigger resume correctly', () => {
        const wrapper = mount(NotificationItem, { store, localVue });

        wrapper.find('.notification-wrap').trigger('mouseover');
        wrapper.find('.notification-wrap').trigger('mouseleave');

        expect(resumeSpy).toHaveBeenCalledTimes(1);
    });

    it('trigger delete correctly', () => {
        const wrapper = mount(NotificationItem, { store, localVue });

        wrapper.find('.notification-wrap__buttons-group').trigger('click');

        expect(deleteSpy).toHaveBeenCalledTimes(1);
    });
});
