// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import NewProjectArea from '@/components/header/NewProjectArea.vue';

import { appStateModule } from '@/store/modules/appState';
import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { makeProjectsModule } from '@/store/modules/projects';
import { makeUsersModule } from '@/store/modules/users';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { CreditCard } from '@/types/payments';
import { User } from '@/types/users';
import { createLocalVue, mount } from '@vue/test-utils';

import { PaymentsMock } from '../../mock/api/payments';
import { ProjectsApiMock } from '../../mock/api/projects';
import { UsersApiMock } from '../../mock/api/users';

const localVue = createLocalVue();
localVue.use(Vuex);

const user = new User('ownerId');
const usersApi = new UsersApiMock();
usersApi.setMockUser(user);
const usersModule = makeUsersModule(usersApi);
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);

const store = new Vuex.Store({ modules: { usersModule, projectsModule, paymentsModule, appStateModule }});

describe('NewProjectArea', () => {
    it('renders correctly without projects and without payment methods', () => {
        const wrapper = mount(NewProjectArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly without projects and without payment methods with info tooltip', async () => {
        const wrapper = mount(NewProjectArea, {
            store,
            localVue,
        });

        await wrapper.find('.info').trigger('mouseenter');

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly without projects and with payment method', async () => {
        const creditCard = new CreditCard('id', 1, 2000, 'test', '0000', true);
        store.commit(PAYMENTS_MUTATIONS.SET_CREDIT_CARDS, [creditCard]);
        store.commit(APP_STATE_MUTATIONS.SHOW_CREATE_PROJECT_BUTTON);

        const wrapper = mount(NewProjectArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();

        await wrapper.find('.new-project-button-container').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with projects and with payment method', () => {
        store.commit(APP_STATE_MUTATIONS.TOGGLE_NEW_PROJECT_POPUP);
        store.commit(APP_STATE_MUTATIONS.HIDE_CREATE_PROJECT_BUTTON);

        const wrapper = mount(NewProjectArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
