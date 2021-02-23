// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import ProjectSummary from '@/components/project/summary/ProjectSummary.vue';

import { makeApiKeysModule } from '@/store/modules/apiKeys';
import { makeBucketsModule } from '@/store/modules/buckets';
import { makePaymentsModule } from '@/store/modules/payments';
import { makeProjectMembersModule } from '@/store/modules/projectMembers';
import { makeProjectsModule } from '@/store/modules/projects';
import { NotificatorPlugin } from '@/utils/plugins/notificator';
import { createLocalVue, mount } from '@vue/test-utils';

import { ApiKeysMock } from '../../mock/api/apiKeys';
import { BucketsMock } from '../../mock/api/buckets';
import { PaymentsMock } from '../../mock/api/payments';
import { ProjectMembersApiMock } from '../../mock/api/projectMembers';
import { ProjectsApiMock } from '../../mock/api/projects';

const notificationPlugin = new NotificatorPlugin();
const localVue = createLocalVue();

localVue.use(Vuex);
localVue.use(notificationPlugin);

localVue.filter('centsToDollars', (cents: number): string => {
    return `$${(cents / 100).toFixed(2)}`;
});

const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
const bucketsApi = new BucketsMock();
const bucketUsageModule = makeBucketsModule(bucketsApi);
const apiKeysApi = new ApiKeysMock();
const apiKeysModule = makeApiKeysModule(apiKeysApi);
const projectMembersApi = new ProjectMembersApiMock();
const projectMembersModule = makeProjectMembersModule(projectMembersApi);

const store = new Vuex.Store({ modules: { projectsModule, paymentsModule, bucketUsageModule, projectMembersModule, apiKeysModule }});

describe('ProjectSummary.vue', (): void => {
    it('renders correctly', (): void => {
        const wrapper = mount(ProjectSummary, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
