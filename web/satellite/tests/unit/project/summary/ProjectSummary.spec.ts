// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, mount } from '@vue/test-utils';

import { AccessGrantsMock } from '../../mock/api/accessGrants';
import { BucketsMock } from '../../mock/api/buckets';
import { PaymentsMock } from '../../mock/api/payments';
import { ProjectMembersApiMock } from '../../mock/api/projectMembers';
import { ProjectsApiMock } from '../../mock/api/projects';

import { makeAccessGrantsModule } from '@/store/modules/accessGrants';
import { makeBucketsModule } from '@/store/modules/buckets';
import { makePaymentsModule } from '@/store/modules/payments';
import { makeProjectMembersModule } from '@/store/modules/projectMembers';
import { makeProjectsModule } from '@/store/modules/projects';
import { NotificatorPlugin } from '@/utils/plugins/notificator';

import ProjectSummary from '@/components/project/summary/ProjectSummary.vue';

const localVue = createLocalVue();
localVue.use(Vuex);

const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
const bucketsApi = new BucketsMock();
const bucketUsageModule = makeBucketsModule(bucketsApi);
const accessGrantsApi = new AccessGrantsMock();
const accessGrantsModule = makeAccessGrantsModule(accessGrantsApi);
const projectMembersApi = new ProjectMembersApiMock();
const projectMembersModule = makeProjectMembersModule(projectMembersApi);

const store = new Vuex.Store({ modules: { projectsModule, paymentsModule, bucketUsageModule, projectMembersModule, accessGrantsModule } });

localVue.use(new NotificatorPlugin(store));

localVue.filter('centsToDollars', (cents: number): string => {
    return `$${(cents / 100).toFixed(2)}`;
});

describe('ProjectSummary.vue', (): void => {
    it('renders correctly', (): void => {
        const wrapper = mount(ProjectSummary, {
            store,
            localVue,
            propsData: {
                isDataFetching: false,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
