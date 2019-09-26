// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';
import Vuex from 'vuex';

import NotificationItem from '@/components/notifications/NotificationItem.vue';

import { makeNotificationsModule } from '@/store/modules/notifications';
import { DelayedNotification } from '@/types/DelayedNotification';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
import { NOTIFICATION_TYPES } from '@/utils/constants/notification';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();

localVue.use(Vuex);
const pauseSpy = sinon.spy();
const resumeSpy = sinon.spy();
const deleteSpy = sinon.spy();
const notificationModule = makeNotificationsModule();
notificationModule.actions[NOTIFICATION_ACTIONS.PAUSE] = pauseSpy;
notificationModule.actions[NOTIFICATION_ACTIONS.RESUME] = resumeSpy;
notificationModule.actions[NOTIFICATION_ACTIONS.DELETE] = deleteSpy;

const store = new Vuex.Store(notificationModule);

describe('Notification.vue', () => {

    it('renders correctly', () => {
        const wrapper = shallowMount(NotificationItem);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with props', () => {
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
        expect(wrapper.find('.notification-wrap__text-area__message').text()).toMatch(testMessage);

        wrapper.setProps({
            notification: new DelayedNotification(
                jest.fn(),
                NOTIFICATION_TYPES.ERROR,
                testMessage,
            ),
        });

        expect(wrapper).toMatchSnapshot();

        wrapper.setProps({
            notification: new DelayedNotification(
                jest.fn(),
                NOTIFICATION_TYPES.NOTIFICATION,
                testMessage,
            ),
        });

        expect(wrapper).toMatchSnapshot();

        wrapper.setProps({
            notification: new DelayedNotification(
                jest.fn(),
                NOTIFICATION_TYPES.WARNING,
                testMessage,
            ),
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('trigger pause correctly', () => {
        const wrapper = shallowMount(NotificationItem, { store, localVue });

        wrapper.find('.notification-wrap').trigger('mouseover');

        expect(pauseSpy.callCount).toBe(1);
    });

    it('trigger resume correctly', () => {
        const wrapper = shallowMount(NotificationItem, { store, localVue });

        wrapper.find('.notification-wrap').trigger('mouseover');
        wrapper.find('.notification-wrap').trigger('mouseleave');

        expect(resumeSpy.callCount).toBe(1);
    });

    it('trigger delete correctly', () => {
        const wrapper = shallowMount(NotificationItem, { store, localVue });

        wrapper.find('.notification-wrap__buttons-group').trigger('click');

        expect(deleteSpy.callCount).toBe(1);
    });
});
