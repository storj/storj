// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, mount } from '@vue/test-utils';

import { PaymentsMock } from '../../../mock/api/payments';
import { ProjectsApiMock } from '../../../mock/api/projects';
import { UsersApiMock } from '../../../mock/api/users';

import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { makeUsersModule, USER_MUTATIONS } from '@/store/modules/users';
import { ProjectUsageAndCharges } from '@/types/payments';
import { Project } from '@/types/projects';
import { User } from '@/types/users';

import EstimatedCostsAndCredits from '@/components/account/billing/estimatedCostsAndCredits/EstimatedCostsAndCredits.vue';

const localVue = createLocalVue();
localVue.filter('centsToDollars', (cents: number): string => {
    return `USD $${(cents / 100).toFixed(2)}`;
});
localVue.use(Vuex);

const usersApi = new UsersApiMock();
const usersModule = makeUsersModule(usersApi);
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
const store = new Vuex.Store({ modules: { usersModule, projectsModule, paymentsModule } });

const now = new Date(2020, 4, 1, 12 /* avoid timezone issues */, 1, 1, 1);
paymentsModule.state.startDate = now; // TODO: we shouldn't need to do this
paymentsModule.state.endDate = now;

const project = new Project('id', 'projectName', 'projectDescription', 'test', 'testOwnerId', true);
const project1 = new Project('id1', 'projectName1', 'projectDescription1', 'test', 'testOwnerId1', false);
const user = new User('testOwnerId');
const date = new Date(1970, 1, 1);
const projectCharge = new ProjectUsageAndCharges(date, date, 100, 100, 100, 'id', 100, 100, 100);
const projectCharge1 = new ProjectUsageAndCharges(date, date, 100, 100, 100, 'id1', 100, 100, 100);
const {
    SET_PROJECT_USAGE_AND_CHARGES,
    SET_PRICE_SUMMARY,
} = PAYMENTS_MUTATIONS;

describe('EstimatedCostsAndCredits', (): void => {
    beforeEach(() => {
        jest.useFakeTimers('modern');
        jest.setSystemTime(now);
    });
    afterAll(() => {
        jest.useRealTimers();
    });

    it('renders correctly with project and no project usage and charges', async (): Promise<void> => {
        await store.commit(USER_MUTATIONS.SET_USER, user);
        await store.commit(PROJECTS_MUTATIONS.ADD, project);

        const wrapper = mount(EstimatedCostsAndCredits, {
            store,
            localVue,
        });

        await wrapper.setData({ isDataFetching: false });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with project and project usage and charges', async (): Promise<void> => {
        await store.commit(PROJECTS_MUTATIONS.ADD, project);
        await store.commit(SET_PROJECT_USAGE_AND_CHARGES, [projectCharge]);
        await store.commit(SET_PRICE_SUMMARY, [projectCharge]);

        const wrapper = mount(EstimatedCostsAndCredits, {
            store,
            localVue,
        });

        await wrapper.setData({ isDataFetching: false });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with 2 projects and project usage and charges', async (): Promise<void> => {
        await store.commit(PROJECTS_MUTATIONS.ADD, project);
        await store.commit(PROJECTS_MUTATIONS.ADD, project1);
        await store.commit(SET_PROJECT_USAGE_AND_CHARGES, [projectCharge, projectCharge1]);
        await store.commit(SET_PRICE_SUMMARY, [projectCharge, projectCharge1]);

        const wrapper = mount(EstimatedCostsAndCredits, {
            store,
            localVue,
        });

        await wrapper.setData({ isDataFetching: false });

        expect(wrapper).toMatchSnapshot();
    });
});
