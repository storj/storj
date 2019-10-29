// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import ApiKeysCopyPopup from '@/components/apiKeys/ApiKeysCopyPopup.vue';

import { ApiKeysApiGql } from '@/api/apiKeys';
import { makeApiKeysModule } from '@/store/modules/apiKeys';
import { makeNotificationsModule } from '@/store/modules/notifications';
import { NotificatorPlugin } from '@/utils/plugins/notificator';
import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);
const notificationPlugin = new NotificatorPlugin();
localVue.use(notificationPlugin);
const apiKeysApi = new ApiKeysApiGql();
const apiKeysModule = makeApiKeysModule(apiKeysApi);
const notificationsModule = makeNotificationsModule();

const store = new Vuex.Store({ modules: { notificationsModule, apiKeysModule }});

describe('ApiKeysCopyPopup', () => {
    it('renders correctly', () => {
        const wrapper = mount(ApiKeysCopyPopup, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('function onCloseClick works correctly', () => {
        const wrapper = mount(ApiKeysCopyPopup, {
            store,
            localVue,
        });

        wrapper.vm.onCloseClick();

        expect(wrapper.vm.$data.isCopiedButtonShown).toBe(false);
        expect(wrapper.emitted()).toEqual({'closePopup': [[]]});
    });

    it('function onCopyClick works correctly', async () => {
        const wrapper = mount(ApiKeysCopyPopup, {
            store,
            localVue,
        });

        await wrapper.vm.onCopyClick();

        expect(wrapper.vm.$data.isCopiedButtonShown).toBe(true);
    });
});
