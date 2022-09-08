// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, mount } from '@vue/test-utils';
import Vuex from 'vuex';

import { NotificatorPlugin } from '@/utils/plugins/notificator';
import { makeNotificationsModule } from '@/store/modules/notifications';
import { appStateModule } from '@/store/modules/appState';

import AddCardForm from '@/components/account/billing/paymentMethods/AddCardForm.vue';

const localVue = createLocalVue();
localVue.use(Vuex);

const notificationsModule = makeNotificationsModule();
const store = new Vuex.Store({ modules: { appStateModule, notificationsModule } });

localVue.use(new NotificatorPlugin(store));

describe('AddCardForm', () => {
    it('renders correctly', () => {
        const wrapper = mount(AddCardForm, {
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
