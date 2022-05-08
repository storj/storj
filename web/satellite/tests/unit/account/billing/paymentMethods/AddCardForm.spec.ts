// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import AddCardForm from '@/components/account/billing/paymentMethods/AddCardForm.vue';

import { NotificatorPlugin } from '@/utils/plugins/notificator';
import { createLocalVue, mount } from '@vue/test-utils';
import {makeNotificationsModule} from "@/store/modules/notifications";
import Vuex from "vuex";
import {appStateModule} from "@/store/modules/appState";

const localVue = createLocalVue();
localVue.use(Vuex);

const notificationsModule = makeNotificationsModule();
const store = new Vuex.Store({ modules: { appStateModule, notificationsModule }});

localVue.use(new NotificatorPlugin(store));

describe('AddCardForm', () => {
    it('renders correctly', () => {
        const wrapper = mount(AddCardForm, {
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
