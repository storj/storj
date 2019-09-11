// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import ApiKeysArea from '@/components/apiKeys/ApiKeysArea.vue';

import { ApiKeysApiGql } from '@/api/apiKeys';
import { makeApiKeysModule } from '@/store/modules/apiKeys';
import { makeNotificationsModule } from '@/store/modules/notifications';
import { API_KEYS_MUTATIONS } from '@/store/mutationConstants';
import { ApiKey } from '@/types/apiKeys';
import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);
const apiKeysApi = new ApiKeysApiGql();
const apiKeysModule = makeApiKeysModule(apiKeysApi);
const notificationsModule = makeNotificationsModule();
const ADD = API_KEYS_MUTATIONS.ADD;
const store = new Vuex.Store({ modules: { apiKeysModule, notificationsModule }});

describe('ApiKeysArea', () => {
    const apiKey = new ApiKey('testId', 'test', 'test', 'test');
    const apiKey1 = new ApiKey('testId1', 'test1', 'test1', 'test1');

    it('renders correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('function apiKeyList works correctly', () => {
        store.commit(ADD, apiKey);

        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.apiKeyList).toEqual([apiKey]);
    });

    it('action on toggleSelection works correctly', () => {
        store.commit(ADD, apiKey1);

        const wrapper = mount(ApiKeysArea, {
            store,
            localVue
        });

        wrapper.vm.toggleSelection(apiKey1);

        expect(store.getters.selectedAPIKeys.length).toBe(1);
    });

    it('action on onClearSelection works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue
        });

        wrapper.vm.onClearSelection();

        expect(wrapper.vm.$data.isDeleteClicked).toBe(false);
    });

    it('function onCreateApiKeyClick works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.onCreateApiKeyClick();

        expect(wrapper.vm.$data.isNewApiKeyPopupShown).toBe(true);
    });

    it('function onFirstDeleteClick works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.onFirstDeleteClick();

        expect(wrapper.vm.$data.isDeleteClicked).toBe(true);
    });

    it('function apiKeyCountTitle works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.apiKeyCountTitle).toMatch('api key');
    });

    it('function isEmpty works correctly', () => {
        store.commit(ADD, apiKey);

        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.isEmpty).toBe(false);
    });

    it('function isSelected works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.isSelected).toBe(false);
    });

    it('function selectedAPIKeysCount works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.selectedAPIKeysCount).toBe(0);
    });

    it('function headerState works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.headerState).toBe(0);
    });

    it('function apiKeyCountTitle with 2 keys works correctly', () => {
        store.commit(ADD, apiKey);
        store.commit(ADD, apiKey1);

        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.apiKeyCountTitle).toMatch('api keys');
    });

    it('function closeNewApiKeyPopup works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.closeNewApiKeyPopup();

        expect(wrapper.vm.$data.isNewApiKeyPopupShown).toBe(false);
    });

    it('function showCopyApiKeyPopup works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        const testSecret = 'testSecret';

        wrapper.vm.showCopyApiKeyPopup(testSecret);

        expect(wrapper.vm.$data.isCopyApiKeyPopupShown).toBe(true);
        expect(wrapper.vm.$data.apiKeySecret).toMatch('testSecret');
    });

    it('function closeCopyNewApiKeyPopup works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.closeCopyNewApiKeyPopup();

        expect(wrapper.vm.$data.isCopyApiKeyPopupShown).toBe(false);
    });

    it('action on onDelete with name works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.onDelete();

        expect(wrapper.vm.$data.isDeleteClicked).toBe(false);
    });
});
