// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import CreateApiKeyStep from '@/components/onboardingTour/steps/CreateApiKeyStep.vue';

import { makeApiKeysModule } from '@/store/modules/apiKeys';
import { makeProjectsModule } from '@/store/modules/projects';
import { ApiKeysPage } from '@/types/apiKeys';
import { Project } from '@/types/projects';
import { NotificatorPlugin } from '@/utils/plugins/notificator';
import { SegmentioPlugin } from '@/utils/plugins/segment';
import { createLocalVue, mount } from '@vue/test-utils';

import { ApiKeysMock } from '../../mock/api/apiKeys';
import { ProjectsApiMock } from '../../mock/api/projects';

const localVue = createLocalVue();
const notificationPlugin = new NotificatorPlugin();
const segmentioPlugin = new SegmentioPlugin();
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const apiKeysApi = new ApiKeysMock();
const apiKeysModule = makeApiKeysModule(apiKeysApi);
apiKeysApi.setMockApiKeysPage(new ApiKeysPage());
const project = new Project('id', 'projectName', 'projectDescription', 'test', 'testOwnerId', true);
projectsApi.setMockProjects([project]);

localVue.use(Vuex);
localVue.use(notificationPlugin);
localVue.use(segmentioPlugin);

const store = new Vuex.Store({ modules: { projectsModule, apiKeysModule }});

describe('CreateApiKeyStep.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = mount(CreateApiKeyStep, {
            store,
            localVue,
        });

        expect(wrapper.findAll('.disabled').length).toBe(1);
        expect(wrapper).toMatchSnapshot();
    });

    it('create api key works correctly correctly', async (): Promise<void> => {
        const wrapper = mount(CreateApiKeyStep, {
            store,
            localVue,
        });

        await wrapper.vm.setApiKeyName('testName');
        await wrapper.vm.createApiKey();

        expect(wrapper.findAll('.disabled').length).toBe(0);
        expect(wrapper).toMatchSnapshot();
    });

    it('done click works correctly correctly', async (): Promise<void> => {
        const wrapper = mount(CreateApiKeyStep, {
            store,
            localVue,
        });

        await wrapper.find('.done-button').trigger('click');

        expect(wrapper.emitted()).toHaveProperty('setUploadDataState');
    });
});
