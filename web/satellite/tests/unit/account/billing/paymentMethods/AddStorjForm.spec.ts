// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import AddStorjForm from '@/components/account/billing/paymentMethods/AddStorjForm.vue';

import { appStateModule } from '@/store/modules/appState';
import { makeNotificationsModule } from '@/store/modules/notifications';
import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { makeUsersModule, USER_MUTATIONS } from '@/store/modules/users';
import { NOTIFICATION_MUTATIONS } from '@/store/mutationConstants';
import { BillingHistoryItem, BillingHistoryItemStatus, BillingHistoryItemType } from '@/types/payments';
import { Project } from '@/types/projects';
import { User } from '@/types/users';
import { Notificator } from '@/utils/plugins/notificator';
import { SegmentioPlugin } from '@/utils/plugins/segment';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

import { PaymentsMock } from '../../../mock/api/payments';
import { ProjectsApiMock } from '../../../mock/api/projects';
import { UsersApiMock } from '../../../mock/api/users';

const localVue = createLocalVue();
const segmentioPlugin = new SegmentioPlugin();

localVue.use(Vuex);
localVue.use(segmentioPlugin);

const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
const usersApi = new UsersApiMock();
const usersModule = makeUsersModule(usersApi);
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const notificationsModule = makeNotificationsModule();
const store = new Vuex.Store({ modules: { usersModule, paymentsModule, projectsModule, appStateModule, notificationsModule }});
store.commit(USER_MUTATIONS.SET_USER, new User('id', 'name', 'short', 'test@test.test', 'partner', 'pass'));

class NotificatorPlugin {
    public install() {
        localVue.prototype.$notify = new Notificator(store);
    }
}

const notificationsPlugin = new NotificatorPlugin();
localVue.use(notificationsPlugin);

describe('AddStorjForm', () => {
    it('renders correctly', () => {
        const wrapper = mount(AddStorjForm, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('user is unable to add less than 50$ or more than 999999$ for the first time', async () => {
        const wrapper = mount(AddStorjForm, {
            store,
            localVue,
        });

        wrapper.vm.$data.tokenDepositValue = 30;
        await wrapper.vm.onConfirmAddSTORJ();

        expect((store.state as any).notificationsModule.notificationQueue[0].message).toMatch('First deposit amount must be more than $50 and less than $1000000');

        wrapper.vm.$data.tokenDepositValue = 1000000;
        await wrapper.vm.onConfirmAddSTORJ();

        expect((store.state as any).notificationsModule.notificationQueue[1].message).toMatch('First deposit amount must be more than $50 and less than $1000000');
    });

    it('user is able to add less than 50$ after coupon is applied', async () => {
        window.open = jest.fn();
        const billingTransactionItem = new BillingHistoryItem('itemId', 'test', 50, 50,
            BillingHistoryItemStatus.Completed, 'test', new Date(), new Date(), BillingHistoryItemType.Transaction);
        const project = new Project('testId', 'test', 'test', 'test', 'id', true);
        store.commit(NOTIFICATION_MUTATIONS.CLEAR);
        store.commit(PAYMENTS_MUTATIONS.SET_BILLING_HISTORY, [billingTransactionItem]);
        store.commit(PROJECTS_MUTATIONS.ADD, project);
        const wrapper = mount(AddStorjForm, {
            store,
            localVue,
        });

        wrapper.vm.$data.tokenDepositValue = 30;
        await wrapper.vm.onConfirmAddSTORJ();

        expect((store.state as any).notificationsModule.notificationQueue[0].type).toMatch('SUCCESS');
    });

    it('renders correctly after continue To Coin Payments click', () => {
        window.open = jest.fn();
        const wrapper = shallowMount(AddStorjForm, {
            store,
            localVue,
            propsData: {
                isLoading: true,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
