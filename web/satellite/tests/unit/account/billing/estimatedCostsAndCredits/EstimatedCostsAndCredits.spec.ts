// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import EstimatedCostsAndCredits from '@/components/account/billing/estimatedCostsAndCredits/EstimatedCostsAndCredits.vue';

import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { makeUsersModule, USER_MUTATIONS } from '@/store/modules/users';
import { ProjectCharge } from '@/types/payments';
import { Project } from '@/types/projects';
import { User } from '@/types/users';
import { createLocalVue, mount } from '@vue/test-utils';

import { PaymentsMock } from '../../../mock/api/payments';
import { ProjectsApiMock } from '../../../mock/api/projects';
import { UsersApiMock } from '../../../mock/api/users';

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
const store = new Vuex.Store({ modules: { usersModule, projectsModule, paymentsModule }});

const project = new Project('id', 'projectName', 'projectDescription', 'test', 'testOwnerId', true);
const user = new User('testOwnerId');
const date = new Date(1970, 1, 1);
const projectCharge = new ProjectCharge(date, date, 100, 100, 100, 'id', 100, 100, 100);

describe('EstimatedCostsAndCredits', () => {
    it('renders correctly with project and no project charges', () => {
        store.commit(USER_MUTATIONS.SET_USER, user);
        store.commit(PROJECTS_MUTATIONS.ADD, project);

        const wrapper = mount(EstimatedCostsAndCredits, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with project and project charges', () => {
        store.commit(PAYMENTS_MUTATIONS.SET_BALANCE, 5500);
        store.commit(PAYMENTS_MUTATIONS.SET_PROJECT_CHARGES, [projectCharge]);

        const wrapper = mount(EstimatedCostsAndCredits, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
