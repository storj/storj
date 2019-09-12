// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import { ApiKeysApiGql } from '@/api/apiKeys';
import { ProjectsApiGql } from '@/api/projects';
import { makeApiKeysModule } from '@/store/modules/apiKeys';
import { makeProjectsModule } from '@/store/modules/projects';
import { API_KEYS_MUTATIONS } from '@/store/mutationConstants';
import { ApiKey } from '@/types/apiKeys';
import { Project } from '@/types/projects';
import { API_KEYS_ACTIONS } from '@/utils/constants/actionNames';
import { createLocalVue } from '@vue/test-utils';

const Vue = createLocalVue();
const apiKeysApi = new ApiKeysApiGql();
const apiKeysModule = makeApiKeysModule(apiKeysApi);
const { FETCH, CREATE, CLEAR_SELECTION, DELETE, TOGGLE_SELECTION, CLEAR } = API_KEYS_ACTIONS;

const projectsApi = new ProjectsApiGql();
const projectsModule = makeProjectsModule(projectsApi);
const selectedProject = new Project('', '', '', '');
selectedProject.id = '1';
projectsModule.state.selectedProject = selectedProject;

const apiKey = new ApiKey('testId', 'testName', 'testCreatedAt', 'testSecret');
const selectedApiKey = new ApiKey('testtestId', 'testtestName', 'testtestCreatedAt', 'testtestSecret');
selectedApiKey.isSelected = true;

Vue.use(Vuex);

const store = new Vuex.Store({modules: { projectsModule, apiKeysModule } });

const state = (store.state as any).apiKeysModule;

describe('mutations', () => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
    });

    it('add apiKey', () => {
        store.commit(API_KEYS_MUTATIONS.ADD, apiKey);

        expect(state.apiKeys[0].id).toBe(apiKey.id);
        expect(state.apiKeys[0].name).toBe(apiKey.name);
        expect(state.apiKeys[0].createdAt).toBe(apiKey.createdAt);
        expect(state.apiKeys[0].secret).toBe(apiKey.secret);
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
    });

    it('success fetch apiKeys', async () => {
        jest.spyOn(apiKeysApi, 'get').mockReturnValue(
            Promise.resolve([apiKey])
        );

        await store.dispatch(FETCH);

        expect(state.apiKeys[0].id).toBe(apiKey.id);
        expect(state.apiKeys[0].name).toBe(apiKey.name);
        expect(state.apiKeys[0].createdAt).toBe(apiKey.createdAt);
        expect(state.apiKeys[0].secret).toBe(apiKey.secret);
    });

    it('fetch throws an error when api call fails', async () => {
        const apikeys = store.getters.apiKeys;
        jest.spyOn(apiKeysApi, 'get').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(FETCH);
        } catch (error) {
            expect(state.apiKeys).toBe(apikeys);
        }
    });

    it('success create apiKeys', async () => {
        jest.spyOn(apiKeysApi, 'create').mockReturnValue(
            Promise.resolve(apiKey)
        );

        await store.dispatch(CREATE, 'testName');

        expect(state.apiKeys[1].id).toBe(apiKey.id);
        expect(state.apiKeys[1].name).toBe(apiKey.name);
        expect(state.apiKeys[1].createdAt).toBe(apiKey.createdAt);
        expect(state.apiKeys[1].secret).toBe(apiKey.secret);
    });

    it('create throws an error when api call fails', async () => {
        jest.spyOn(apiKeysApi, 'create').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(CREATE, 'testName');
        } catch (error) {
            expect(state.apiKeys).toEqual([apiKey, apiKey]);
        }
    });

    it('success delete apiKeys', async () => {
        jest.spyOn(apiKeysApi, 'delete').mockReturnValue(
            Promise.resolve(null)
        );

        await store.dispatch(DELETE, ['testId', 'testId']);

        expect(state.apiKeys).toEqual([]);
    });

    it('delete throws an error when api call fails', async () => {
        jest.spyOn(apiKeysApi, 'delete').mockImplementation(() => { throw new Error(); });

        store.commit(API_KEYS_MUTATIONS.ADD, apiKey);

        try {
            await store.dispatch(DELETE, 'testId');
        } catch (error) {
            expect(state.apiKeys).toEqual([apiKey]);
        }
    });

    it('success toggleAPIKeySelection apiKeys', () => {
        store.commit(API_KEYS_MUTATIONS.ADD, selectedApiKey);

        store.dispatch(TOGGLE_SELECTION, ['testId']);

        expect(store.getters.selectedAPIKeys).toEqual([selectedApiKey]);
    });

    it('success clearSelection apiKeys', () => {
        store.dispatch(CLEAR_SELECTION);

        expect(store.getters.selectedAPIKeys).toEqual([]);
    });

    it('success clearAPIKeys', () => {
        store.dispatch(CLEAR);

        expect(state.apiKeys).toEqual([]);
    });
});

describe('getters', () => {
    const selectedApiKey = new ApiKey('testtestId', 'testtestName', 'testtestCreatedAt', 'testtestSecret');
    selectedApiKey.isSelected = true;

    it('selected apiKeys', () => {
        store.commit(API_KEYS_MUTATIONS.ADD, selectedApiKey);

        const retrievedApiKeys = store.getters.selectedAPIKeys;

        expect(retrievedApiKeys[0].id).toBe('testtestId');
    });

    it('apiKeys array', () => {
        const retrievedApiKeys = store.getters.selectedAPIKeys;

        expect(retrievedApiKeys).toEqual([selectedApiKey]);
    });
});
