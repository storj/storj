// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import ApiKeysCreationPopup from '@/components/apiKeys/ApiKeysCreationPopup.vue';

import { ApiKeysApiGql } from '@/api/apiKeys';
import { ProjectsApiGql } from '@/api/projects';
import { API_KEYS_ACTIONS, makeApiKeysModule } from '@/store/modules/apiKeys';
import { makeNotificationsModule } from '@/store/modules/notifications';
import { makeProjectsModule } from '@/store/modules/projects';
import { ApiKey } from '@/types/apiKeys';
import { Project } from '@/types/projects';
import { NotificatorPlugin } from '@/utils/plugins/notificator';
import { SegmentioPlugin } from '@/utils/plugins/segment';
import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);
const notificationPlugin = new NotificatorPlugin();
const segmentioPlugin = new SegmentioPlugin();
localVue.use(notificationPlugin);
localVue.use(segmentioPlugin);
const apiKeysApi = new ApiKeysApiGql();
const apiKeysModule = makeApiKeysModule(apiKeysApi);
const projectsApi = new ProjectsApiGql();
const projectsModule = makeProjectsModule(projectsApi);
const notificationsModule = makeNotificationsModule();

const selectedProject = new Project();
selectedProject.id = '1';

projectsModule.state.selectedProject = selectedProject;

const CREATE = API_KEYS_ACTIONS.CREATE;
const store = new Vuex.Store({ modules: { projectsModule, apiKeysModule, notificationsModule }});

describe('ApiKeysCreationPopup', (): void => {
    const value = 'testValue';

    it('renders correctly', (): void => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
            propsData: {
                isPopupShown: true,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('function onCloseClick works correctly', async (): Promise<void> => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        await wrapper.vm.onCloseClick();

        expect(wrapper.emitted()).toEqual({'closePopup': [[]]});
    });

    it('function onChangeName works correctly', async (): Promise<void> => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        await wrapper.vm.onChangeName(value);

        wrapper.vm.$data.name = value.trim();
        expect(wrapper.vm.$data.name).toMatch('testValue');
        expect(wrapper.vm.$data.errorMessage).toMatch('');
    });

    it('action on onNextClick with no name works correctly', async (): Promise<void> => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        wrapper.vm.$data.isLoading = false;
        wrapper.vm.$data.name = '';

        await wrapper.vm.onNextClick();

        expect(wrapper.vm.$data.errorMessage).toMatch('API Key name can`t be empty');
    });

    it('action on onNextClick with name works correctly', async (): Promise<void> => {
        const date = new Date(0);
        const testApiKey = new ApiKey('testId', 'testName', date, 'test');

        jest.spyOn(apiKeysApi, 'create').mockReturnValue(
            Promise.resolve(testApiKey));

        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        wrapper.vm.$data.isLoading = false;
        wrapper.vm.$data.name = 'testName';

        await wrapper.vm.onNextClick();

        const result = await store.dispatch(CREATE, 'testName');

        expect(wrapper.vm.$data.key).toBe(result.secret);
        expect(wrapper.vm.$data.isLoading).toBe(false);
        expect(wrapper.emitted()).toEqual({'closePopup': [[]], 'showCopyPopup': [['test']]});
    });
});
