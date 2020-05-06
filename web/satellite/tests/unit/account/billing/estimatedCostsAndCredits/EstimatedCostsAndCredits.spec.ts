// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import EstimatedCostsAndCredits from '@/components/account/billing/estimatedCostsAndCredits/EstimatedCostsAndCredits.vue';

import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { makeUsersModule, USER_MUTATIONS } from '@/store/modules/users';
import { BillingHistoryItem, BillingHistoryItemType, ProjectUsageAndCharges } from '@/types/payments';
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
const projectCharge = new ProjectUsageAndCharges(date, date, 100, 100, 100, 'id', 100, 100, 100);
const {
    SET_BALANCE,
    CLEAR,
    SET_BILLING_HISTORY,
    SET_PROJECT_USAGE_AND_CHARGES,
    SET_CURRENT_ROLLUP_PRICE,
    SET_PREVIOUS_ROLLUP_PRICE,
    SET_PRICE_SUMMARY,
} = PAYMENTS_MUTATIONS;

describe('EstimatedCostsAndCredits', () => {
    it('renders correctly with project and no project usage and charges', () => {
        store.commit(USER_MUTATIONS.SET_USER, user);
        store.commit(PROJECTS_MUTATIONS.ADD, project);

        const wrapper = mount(EstimatedCostsAndCredits, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with project and project usage and charges', () => {
        store.commit(SET_BALANCE, 5500);
        store.commit(SET_PROJECT_USAGE_AND_CHARGES, [projectCharge]);
        store.commit(SET_PRICE_SUMMARY, [projectCharge]);
        store.commit(SET_CURRENT_ROLLUP_PRICE);
        store.commit(SET_PREVIOUS_ROLLUP_PRICE);

        const wrapper = mount(EstimatedCostsAndCredits, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    // TODO: use when coupon expiration bug is fixed
    // it('renders correctly with project and project usage and charges with previous rollup invoice', () => {
    //     const now = new Date();
    //     let billingHistoryItem = new BillingHistoryItem('id', 'description', 300, 300, 'paid', 'test', new Date(now.getUTCFullYear(), now.getUTCMonth() - 1, 15), now, BillingHistoryItemType.Invoice);
    //
    //     if (now.getUTCMonth() === 0) {
    //         billingHistoryItem = new BillingHistoryItem('id', 'description', 300, 300, 'paid', 'test', new Date(now.getUTCFullYear() - 1, 11, 15), now, BillingHistoryItemType.Invoice);
    //     }
    //
    //     store.commit(CLEAR);
    //     store.commit(SET_BALANCE, 600);
    //     store.commit(SET_PROJECT_USAGE_AND_CHARGES, [projectCharge]);
    //     store.commit(SET_PRICE_SUMMARY, [projectCharge]);
    //     store.commit(SET_CURRENT_ROLLUP_PRICE);
    //     store.commit(SET_BILLING_HISTORY, [billingHistoryItem]);
    //
    //     const wrapper = mount(EstimatedCostsAndCredits, {
    //         store,
    //         localVue,
    //     });
    //
    //     expect(wrapper).toMatchSnapshot();
    // });
    //
    // it('renders correctly with project and project usage and charges with price bigger than balance amount', () => {
    //     store.commit(CLEAR);
    //     store.commit(SET_BALANCE, 500);
    //     store.commit(SET_PROJECT_USAGE_AND_CHARGES, [projectCharge]);
    //     store.commit(SET_PRICE_SUMMARY, [projectCharge]);
    //     store.commit(SET_CURRENT_ROLLUP_PRICE);
    //     store.commit(SET_PREVIOUS_ROLLUP_PRICE);
    //     store.commit(SET_BILLING_HISTORY, []);
    //
    //     const wrapper = mount(EstimatedCostsAndCredits, {
    //         store,
    //         localVue,
    //     });
    //
    //     expect(wrapper).toMatchSnapshot();
    // });
});
