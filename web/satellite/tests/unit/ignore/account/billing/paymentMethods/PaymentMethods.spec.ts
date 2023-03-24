// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

import { PaymentsMock } from '../../../../mock/api/payments';
import { ProjectsApiMock } from '../../../../mock/api/projects';
import { UsersApiMock } from '../../../../mock/api/users';
import { FrontendConfigApiMock } from '../../../../mock/api/config';

import { makeAppStateModule } from '@/store/modules/appState';
import { makeNotificationsModule } from '@/store/modules/notifications';
import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { makeProjectsModule } from '@/store/modules/projects';
import { makeUsersModule, USER_MUTATIONS } from '@/store/modules/users';
import { CreditCard } from '@/types/payments';
import { User } from '@/types/users';
import { NotificatorPlugin } from '@/utils/plugins/notificator';

import PaymentMethods from '@/components/account/billing/paymentMethods/PaymentMethods.vue';

const localVue = createLocalVue();

localVue.use(Vuex);

const appStateModule = makeAppStateModule(new FrontendConfigApiMock());
const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
const usersApi = new UsersApiMock();
const usersModule = makeUsersModule(usersApi);
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const notificationsModule = makeNotificationsModule();
const store = new Vuex.Store({ modules: { usersModule, paymentsModule, projectsModule, appStateModule, notificationsModule } });
store.commit(USER_MUTATIONS.SET_USER, new User('id', 'name', 'short', 'test@test.test', 'partner', 'pass'));

localVue.use(new NotificatorPlugin(store));

const ANIMATION_COMPLETE_TIME = 600;

describe('PaymentMethods', () => {
    it('renders correctly without card', async (): Promise<void> => {
        const wrapper = shallowMount(PaymentMethods, {
            store,
            localVue,
        });

        await wrapper.setData({ areCardsFetching: false });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly after add card and cancel click', async () => {
        const wrapper = shallowMount(PaymentMethods, {
            store,
            localVue,
        });

        await wrapper.setData({ areCardsFetching: false });

        await wrapper.find('.add-card-button').trigger('click');

        await new Promise<void>(resolve => {
            setTimeout(() => {
                expect(wrapper).toMatchSnapshot();
                wrapper.find('.payment-methods-area__functional-area__button-area__cancel').trigger('click');

                setTimeout(() => {
                    expect(wrapper).toMatchSnapshot();
                    resolve();
                }, ANIMATION_COMPLETE_TIME);
            }, ANIMATION_COMPLETE_TIME);
        });
    });

    it('renders correctly after add STORJ and cancel click', async () => {
        const wrapper = mount(PaymentMethods, {
            store,
            localVue,
        });

        await wrapper.setData({ areCardsFetching: false });

        await wrapper.find('.add-storj-button').trigger('click');

        await new Promise<void>(resolve => {
            setTimeout(() => {
                expect(wrapper).toMatchSnapshot();
                wrapper.find('.payment-methods-area__functional-area__button-area__cancel').trigger('click');

                setTimeout(() => {
                    expect(wrapper).toMatchSnapshot();
                    resolve();
                }, ANIMATION_COMPLETE_TIME);
            }, ANIMATION_COMPLETE_TIME);
        });
    });

    it('renders correctly with card', async (): Promise<void> => {
        const card = new CreditCard('cardId', 12, 2100, 'test', '0000', true);
        store.commit(PAYMENTS_MUTATIONS.SET_CREDIT_CARDS, [card]);

        const wrapper = shallowMount(PaymentMethods, {
            store,
            localVue,
        });

        await wrapper.setData({ areCardsFetching: false });

        expect(wrapper).toMatchSnapshot();
    });
});
