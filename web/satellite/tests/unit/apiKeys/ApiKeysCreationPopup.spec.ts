// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, mount } from '@vue/test-utils';
import Vuex from 'vuex';
import ApiKeysCreationPopup from '@/components/apiKeys/ApiKeysCreationPopup.vue';
import * as apiKeysApi from '@/api/apiKeys';
import { ApiKey } from '@/types/apiKeys';
import { makeApiKeysModule } from '@/store/modules/apiKeys';
import { makeProjectsModule } from '@/store/modules/projects';
import { API_KEYS_ACTIONS } from '@/utils/constants/actionNames';
import { Project } from '@/types/projects';
import { RequestResponse } from '@/types/response';

const localVue = createLocalVue();
localVue.use(Vuex);
const apiKeysModule = makeApiKeysModule();
const projectsModule = makeProjectsModule();

const selectedProject = new Project();
selectedProject.id = '1';

projectsModule.state.selectedProject = selectedProject;

const store = new Vuex.Store({modules: { projectsModule, apiKeysModule }});

describe('ApiKeysCreationPopup', () => {
    let value = 'testValue';

    it('renders correctly', () => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('function onCloseClick works correctly', () => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        wrapper.vm.onCloseClick();

        expect(wrapper.emitted()).toEqual({'closePopup': [[]]});
    });

    it('function onChangeName works correctly', () => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        wrapper.vm.onChangeName(value);

        wrapper.vm.$data.name = value.trim();
        expect(wrapper.vm.$data.name).toMatch('testValue');
        expect(wrapper.vm.$data.errorMessage).toMatch('');
    });

    it('action on onNextClick with no name works correctly', async () => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        wrapper.vm.$data.isLoading = false;
        wrapper.vm.$data.name = '';

        await wrapper.vm.onNextClick();

        expect(wrapper.vm.$data.errorMessage).toMatch('API Key name can`t be empty');
    });

    it('action on onNextClick with name works correctly', async () => {
        jest.spyOn(apiKeysApi, 'createAPIKey').mockReturnValue(
            Promise.resolve(<RequestResponse<ApiKey>>{
                isSuccess: true,
                data: {id: 'testId',  secret: 'testSecret',  name: 'testName',  createdAt: 'test'}, errorMessage: ''})
        );

        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        wrapper.vm.$data.isLoading = false;
        wrapper.vm.$data.name = 'testName';

        wrapper.vm.onNextClick();

        let result = await store.dispatch(API_KEYS_ACTIONS.CREATE, 'testName');

        expect(wrapper.vm.$data.key).toBe(result.data.secret);
        expect(wrapper.vm.$data.isLoading).toBe(false);
        expect(wrapper.emitted()).toEqual({'closePopup': [[]], 'showCopyPopup': [['testSecret']]});
    });
});
