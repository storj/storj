// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, mount, shallowMount } from '@vue/test-utils';
import Vuex from 'vuex';
import sinon from 'sinon';
import Notification from '@/components/notifications/Notification.vue';
import { NOTIFICATION_TYPES } from '@/utils/constants/notification';
import { DelayedNotification } from '@/types/DelayedNotification';

const localVue = createLocalVue();

localVue.use(Vuex);

describe('Notification.vue', () => {
    let actions;
    let store;
    const pauseSpy = sinon.spy();
    const resumeSpy = sinon.spy();
    const deleteSpy = sinon.spy();

    beforeEach(() => {
        actions = {
            pauseNotification: pauseSpy,
            resumeNotification: resumeSpy,
            deleteNotification: deleteSpy,
        };

        store = new Vuex.Store({
            modules: {
                notificationsModule: {
                    actions
                }
            }
        });
    });

    it('renders correctly', () => {
        const wrapper = shallowMount(Notification);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with props', () => {
        const testMessage = 'testMessage';

        const wrapper = mount(Notification, {
            propsData: {
                notification: new DelayedNotification(
                    jest.fn(),
                    NOTIFICATION_TYPES.SUCCESS,
                    testMessage
                )
            },
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.find('.notification-wrap__text').text()).toMatch(testMessage);

        wrapper.setProps({
            notification: new DelayedNotification(
                jest.fn(),
                NOTIFICATION_TYPES.ERROR,
                testMessage
            )
        });

        expect(wrapper).toMatchSnapshot();

        wrapper.setProps({
            notification: new DelayedNotification(
                jest.fn(),
                NOTIFICATION_TYPES.NOTIFICATION,
                testMessage
            )
        });

        expect(wrapper).toMatchSnapshot();

        wrapper.setProps({
            notification: new DelayedNotification(
                jest.fn(),
                NOTIFICATION_TYPES.WARNING,
                testMessage
            )
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('trigger pause correctly', () => {
        const wrapper = shallowMount(Notification, { store, localVue });

        wrapper.find('.notification-wrap').trigger('mouseover');

        expect(pauseSpy.callCount).toBe(1);
    });

    it('trigger resume correctly', () => {
        const wrapper = shallowMount(Notification, { store, localVue });

        wrapper.find('.notification-wrap').trigger('mouseover');
        wrapper.find('.notification-wrap').trigger('mouseleave');

        expect(resumeSpy.callCount).toBe(1);
    });

    it('trigger delete correctly', () => {
        const wrapper = shallowMount(Notification, { store, localVue });

        wrapper.find('.notification-wrap__buttons-group').trigger('click');

        expect(deleteSpy.callCount).toBe(1);
    });
});
