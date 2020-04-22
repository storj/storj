// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import PayingStep from '@/components/onboardingTour/steps/paymentStates/tokenSubSteps/PayingStep.vue';

import { appStateModule } from '@/store/modules/appState';
import { makeNotificationsModule } from '@/store/modules/notifications';
import { makePaymentsModule } from '@/store/modules/payments';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { Project } from '@/types/projects';
import { Notificator } from '@/utils/plugins/notificator';
import { SegmentioPlugin } from '@/utils/plugins/segment';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

import { PaymentsMock } from '../../../../mock/api/payments';
import { ProjectsApiMock } from '../../../../mock/api/projects';

const localVue = createLocalVue();
const segmentioPlugin = new SegmentioPlugin();
localVue.use(Vuex);
localVue.use(segmentioPlugin);

const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const notificationsModule = makeNotificationsModule();
const store = new Vuex.Store({ modules: { paymentsModule, notificationsModule, appStateModule, projectsModule }});

class NotificatorPlugin {
    public install() {
        localVue.prototype.$notify = new Notificator(store);
    }
}

const notificationsPlugin = new NotificatorPlugin();
localVue.use(notificationsPlugin);

describe('PayingStep.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(PayingStep, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('user is unable to add less than 50$ or more than 999999$', async (): Promise<void> => {
        const wrapper = mount(PayingStep, {
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

    it('continue to coin payments works correctly', async (): Promise<void> => {
        const project = new Project('testId', 'test', 'test', 'test', 'id', true);
        store.commit(PROJECTS_MUTATIONS.ADD, project);
        window.open = jest.fn();
        const wrapper = mount(PayingStep, {
            store,
            localVue,
        });

        wrapper.vm.$data.tokenDepositValue = 70;
        await wrapper.vm.onConfirmAddSTORJ();

        expect(wrapper.vm.$data.tokenDepositValue).toEqual(50);
    });
});
