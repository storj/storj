// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { AccessGrantsMock } from '../mock/api/accessGrants';
import { BucketsMock } from '../mock/api/buckets';
import { PaymentsMock } from '../mock/api/payments';
import { ProjectMembersApiMock } from '../mock/api/projectMembers';
import { ProjectsApiMock } from '../mock/api/projects';

import { makeAccessGrantsModule } from '@/store/modules/accessGrants';
import { appStateModule } from '@/store/modules/appState';
import { makeBucketsModule } from '@/store/modules/buckets';
import { makePaymentsModule } from '@/store/modules/payments';
import { makeProjectMembersModule } from '@/store/modules/projectMembers';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { AccessGrantsPage } from '@/types/accessGrants';
import { ProjectMembersPage } from '@/types/projectMembers';
import { Project } from '@/types/projects';

import ProjectDashboard from '@/components/project/ProjectDashboard.vue';

const localVue = createLocalVue();
localVue.use(Vuex);

const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const bucketsApi = new BucketsMock();
const bucketsModule = makeBucketsModule(bucketsApi);
const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
const membersApi = new ProjectMembersApiMock();
const membersModule = makeProjectMembersModule(membersApi);
const grantsApi = new AccessGrantsMock();
const grantsModule = makeAccessGrantsModule(grantsApi);

const store = new Vuex.Store({ modules: { appStateModule, projectsModule, bucketsModule, paymentsModule, membersModule, grantsModule } });

const project = new Project('id', 'test', 'test', 'test', 'ownedId', false);
const membersPage = new ProjectMembersPage();
membersApi.setMockPage(membersPage);
const grantsPage = new AccessGrantsPage();
grantsApi.setMockAccessGrantsPage(grantsPage);

describe('ProjectDashboard.vue', () => {
    it('renders correctly', async (): Promise<void> => {
        store.commit(PROJECTS_MUTATIONS.ADD, project);
        store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, project.id);

        const wrapper = shallowMount(ProjectDashboard, {
            store,
            localVue,
        });

        await wrapper.setData({
            areBucketsFetching: false,
            isSummaryDataFetching: false,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
