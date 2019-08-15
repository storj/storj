// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, mount } from '@vue/test-utils';
import Vuex from 'vuex';
import ApiKeysArea from '@/components/apiKeys/ApiKeysArea.vue';
import { ApiKey } from '@/types/apiKeys';
import { apiKeysModule } from '@/store/modules/apiKeys';
import { API_KEYS_MUTATIONS } from '@/store/mutationConstants';

const localVue = createLocalVue();

localVue.use(Vuex);

let state = apiKeysModule.state;
let mutations = apiKeysModule.mutations;
let actions = apiKeysModule.actions;
let getters = apiKeysModule.getters;

const store = new Vuex.Store({
    modules: {
        apiKeysModule: {
            state,
            mutations,
            actions,
            getters
        }
    }
});

describe('ApiKeysArea', () => {
    let apiKey = new ApiKey('testId', 'test', 'test', 'test');
    let apiKey1 = new ApiKey('testId1', 'test1', 'test1', 'test1');

    it('renders correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('function apiKeyList works correctly', () => {
        store.commit(API_KEYS_MUTATIONS.ADD, apiKey);

        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.apiKeyList).toEqual([apiKey]);
    });

    it('action on toggleSelection works correctly', () => {
        store.commit(API_KEYS_MUTATIONS.ADD, apiKey1);

        const wrapper = mount(ApiKeysArea, {
            store,
            localVue
        });

        wrapper.vm.toggleSelection(apiKey1.id);

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
        store.commit(API_KEYS_MUTATIONS.ADD, apiKey);

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
        store.commit(API_KEYS_MUTATIONS.ADD, apiKey);
        store.commit(API_KEYS_MUTATIONS.ADD, apiKey1);

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

        let testSecret = 'testSecret';

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
});

describe('ApiKeysArea async success', () => {
    let store;
    let actions;
    let state;
    let getters;
    let apiKey = new ApiKey('testId', 'test', 'test', 'test');

    beforeEach(() => {
        actions = {
            deleteAPIKey: async () => {
                return {
                    errorMessage: '',
                    isSuccess: true,
                    data: null
                };
            },
            success: jest.fn()
        };

        getters = {
            selectedAPIKeys: () => [apiKey]
        };

        state = {
            apiKeys: [apiKey]
        };

        store = new Vuex.Store({
            modules: {
                apiKeysModule: {
                    state,
                    actions,
                    getters
                }
            }
        });
    });

    it('action on onDelete with name works correctly', async () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.onDelete();

        await actions.deleteAPIKey();

        expect(wrapper.vm.$data.isDeleteClicked).toBe(false);
        expect(actions.success.mock.calls).toHaveLength(1);
    });
});

describe('ApiKeysArea async not success', () => {
    let store;
    let actions;
    let state;
    let getters;

    beforeEach(() => {
        actions = {
            deleteAPIKey: async () => {
                return {
                    errorMessage: '',
                    isSuccess: false,
                    data: null
                };
            },
            error: jest.fn()
        };

        getters = {
            selectedAPIKeys: () => []
        };

        state = {
            apiKeys: []
        };

        store = new Vuex.Store({
            modules: {
                apiKeysModule: {
                    state,
                    actions,
                    getters
                }
            }
        });
    });

    it('action on onDelete with name works correctly', async () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.onDelete();

        await actions.deleteAPIKey();

        expect(actions.error.mock.calls).toHaveLength(1);
    });
});
