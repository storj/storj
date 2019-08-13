// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, mount } from '@vue/test-utils';
import * as sinon from 'sinon';
import Vuex from 'vuex';
import ApiKeysArea from '@/components/apiKeys/ApiKeysArea.vue';
import { ApiKey } from '@/types/apiKeys';

const localVue = createLocalVue();

localVue.use(Vuex);

describe('ApiKeysArea', () => {
    let store;
    let actions;
    let state;
    let getters;
    let apiKey = new ApiKey('testId', 'test', 'test', 'test');
    let createAPIKeySpy = sinon.spy();
    let deleteAPIKeySpy = sinon.spy();
    let value = 'testValue';

    beforeEach(() => {
        actions = {
            fetchAPIKeys: jest.fn(),
            toggleAPIKeySelection: jest.fn(),
            clearAPIKeySelection: jest.fn(),
            deleteAPIKey: deleteAPIKeySpy,
            createAPIKey: createAPIKeySpy,
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

    it('renders correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('action on toggleSelection works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue
        });

        wrapper.vm.toggleSelection(apiKey.id);

        expect(actions.fetchAPIKeys.mock.calls).toHaveLength(1);
        expect(actions.toggleAPIKeySelection.mock.calls).toHaveLength(1);
    });

    it('action on onClearSelection works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue
        });

        wrapper.vm.onClearSelection();

        expect(actions.clearAPIKeySelection.mock.calls).toHaveLength(1);
        expect(wrapper.vm.$data.isDeleteClicked).toBe(false);
    });

    it('action on onNextClick works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.$data.isLoading = false;
        wrapper.vm.$data.isNewApiKeyPopupShown = true;
        wrapper.vm.$data.name = 'testName';

        wrapper.find('.next-button').trigger('click');

        expect(createAPIKeySpy.callCount).toBe(1);
    });

    it('action on onDelete works correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.$data.isDeleteClicked = true;

        wrapper.find('.deletion').trigger('click');

        expect(deleteAPIKeySpy.callCount).toBe(1);
    });

    it('function onCreateApiKeyClick work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.onCreateApiKeyClick();

        expect(wrapper.vm.$data.isNewApiKeyPopupShown).toBe(true);
    });

    it('function onFirstDeleteClick work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.onFirstDeleteClick();

        expect(wrapper.vm.$data.isDeleteClicked).toBe(true);
    });

    it('function onCloseClick work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.onCloseClick();

        expect(wrapper.vm.$data.isNewApiKeyPopupShown).toBe(false);
        expect(wrapper.vm.$data.isCopiedButtonShown).toBe(false);
        expect(wrapper.vm.$data.isCopyApiKeyPopupShown).toBe(false);
    });

    it('function onChangeName work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.onChangeName(value);

        wrapper.vm.$data.name = value.trim();
        expect(wrapper.vm.$data.name).toMatch('testValue');
        expect(wrapper.vm.$data.errorMessage).toMatch('');
    });

    it('function onCopyClick work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.onCopyClick();

        expect(actions.success.mock.calls).toHaveLength(1);
        expect(wrapper.vm.$data.isCopiedButtonShown).toBe(true);
    });

    it('function apiKeyCountTitle work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.apiKeyCountTitle).toMatch('api key');
    });

    it('function apiKeyList work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.apiKeyList).toEqual([apiKey]);
    });

    it('function isEmpty work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.isEmpty).toBe(false);
    });

    it('function isSelected work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.isSelected).toBe(true);
    });

    it('function selectedAPIKeysCount work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.selectedAPIKeysCount).toBe(1);
    });

    it('function headerState work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.headerState).toBe(1);
    });
});

describe('ApiKeysArea without ApiKeys', () => {
    let store;
    let state;
    let getters;

    beforeEach(() => {
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
                    getters
                }
            }
        });
    });

    it('function selectedAPIKeysCount work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.selectedAPIKeysCount).toBe(0);
    });

    it('function headerState work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.headerState).toBe(0);
    });
});

describe('ApiKeysArea with 2 ApiKeys', () => {
    let store;
    let state;
    let getters;
    let apiKey = new ApiKey('testId', 'test', 'test', 'test');
    let apiKey1 = new ApiKey('testId1', 'test1', 'test1', 'test1');

    beforeEach(() => {
        getters = {
            selectedAPIKeys: () => [apiKey, apiKey1]
        };

        state = {
            apiKeys: [apiKey, apiKey1]
        };

        store = new Vuex.Store({
            modules: {
                apiKeysModule: {
                    state,
                    getters
                }
            }
        });
    });

    it('function apiKeyCountTitle work correctly', () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.apiKeyCountTitle).toMatch('api keys');
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
            fetchAPIKeys: jest.fn(),
            toggleAPIKeySelection: jest.fn(),
            clearAPIKeySelection: jest.fn(),
            deleteAPIKey: async () => {
                return {
                    errorMessage: '',
                    isSuccess: true,
                    data: null
                };
            },

            createAPIKey: async () => {
                return {
                    errorMessage: '',
                    isSuccess: true,
                    data: apiKey,
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

    it('action on onNextClick with no name works correctly', async () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.$data.isLoading = false;
        wrapper.vm.$data.name = '';

        await wrapper.vm.onNextClick();

        expect(wrapper.vm.$data.errorMessage).toMatch('API Key name can`t be empty');
    });

    it('action on onNextClick with name works correctly', async () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.$data.isLoading = false;
        wrapper.vm.$data.name = 'testName';

        wrapper.vm.onNextClick();

        let result = await actions.createAPIKey();

        expect(actions.success.mock.calls).toHaveLength(1);
        expect(wrapper.vm.$data.key).toBe(result.data.secret);
        expect(wrapper.vm.$data.isLoading).toBe(false);
        expect(wrapper.vm.$data.isNewApiKeyPopupShown).toBe(false);
        expect(wrapper.vm.$data.isCopyApiKeyPopupShown).toBe(true);
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
    let apiKey = new ApiKey('testId', 'test', 'test', 'test');

    beforeEach(() => {
        actions = {
            fetchAPIKeys: jest.fn(),
            toggleAPIKeySelection: jest.fn(),
            clearAPIKeySelection: jest.fn(),
            deleteAPIKey: async () => {
                return {
                    errorMessage: '',
                    isSuccess: false,
                    data: null
                };
            },

            createAPIKey: async () => {
                return {
                    errorMessage: '',
                    isSuccess: false,
                    data: null,
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

    it('action on onNextClick while loading works correctly', async () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.$data.isLoading = true;

        wrapper.vm.onNextClick();

        expect(wrapper.vm.$data.isLoading).toBe(true);
    });

    it('action on onNextClick works correctly', async () => {
        const wrapper = mount(ApiKeysArea, {
            store,
            localVue,
        });

        wrapper.vm.$data.isLoading = false;
        wrapper.vm.$data.name = 'testName';

        wrapper.vm.onNextClick();

        await actions.createAPIKey();

        expect(actions.error.mock.calls).toHaveLength(1);
        expect(wrapper.vm.$data.isLoading).toBe(false);
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
