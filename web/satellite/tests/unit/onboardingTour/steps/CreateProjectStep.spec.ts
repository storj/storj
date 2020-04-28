// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';
import Vuex from 'vuex';

import CreateProjectStep from '@/components/onboardingTour/steps/CreateProjectStep.vue';

import { PaymentsHttpApi } from '@/api/payments';
import { makePaymentsModule } from '@/store/modules/payments';
import { makeProjectsModule } from '@/store/modules/projects';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../../mock/api/projects';

const localVue = createLocalVue();
localVue.use(Vuex);
const paymentsApi = new PaymentsHttpApi();
const paymentsModule = makePaymentsModule(paymentsApi);
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);

const store = new Vuex.Store({ modules: { paymentsModule, projectsModule }});

describe('CreateProjectStep.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(CreateProjectStep, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('click works correctly', async (): Promise<void> => {
        const clickSpy = sinon.spy();
        const wrapper = mount(CreateProjectStep, {
            store,
            localVue,
            methods: {
                createProjectClick: clickSpy,
            },
        });

        expect(wrapper.findAll('.disabled').length).toBe(1);

        await wrapper.vm.setProjectName('test');

        expect(wrapper.findAll('.disabled').length).toBe(0);

        await wrapper.find('.create-project-button').trigger('click');

        expect(clickSpy.callCount).toBe(1);
    });
});
