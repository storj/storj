// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { VNode } from 'vue';
import { DirectiveBinding } from 'vue/types/options';
import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../../../../mock/api/projects';
import { PaymentsMock } from '../../../../mock/api/payments';
import { FrontendConfigApiMock } from '../../../../mock/api/config';

import { makeAppStateModule } from '@/store/modules/appState';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { Project } from '@/types/projects';
import { makePaymentsModule } from '@/store/modules/payments';

import PeriodSelection from '@/components/account/billing/depositAndBillingHistory/PeriodSelection.vue';

const localVue = createLocalVue();
const appStateModule = makeAppStateModule(new FrontendConfigApiMock());
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
const project = new Project('id', 'projectName', 'projectDescription', 'test', 'testOwnerId', false);

let clickOutsideEvent: EventListener;

localVue.directive('click-outside', {
    bind: function (el: HTMLElement, binding: DirectiveBinding, vnode: VNode) {
        clickOutsideEvent = function(event: Event): void {
            if (el === event.target) {
                return;
            }

            if (vnode.context && binding.expression) {
                vnode.context[binding.expression](event);
            }
        };

        document.body.addEventListener('click', clickOutsideEvent);
    },
    unbind: function(): void {
        document.body.removeEventListener('click', clickOutsideEvent);
    },
});

localVue.use(Vuex);

const store = new Vuex.Store({ modules: {
    projectsModule,
    appStateModule,
    paymentsModule,
} });
store.commit(PROJECTS_MUTATIONS.SET_PROJECTS, [project]);
store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, project.id);

describe('PeriodSelection', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(PeriodSelection, {
            localVue,
            store,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with dropdown', async (): Promise<void> => {
        const wrapper = shallowMount(PeriodSelection, {
            localVue,
            store,
        });

        await wrapper.find('.period-selection').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });

    it('clicks work correctly', async (): Promise<void> => {
        const currentClickSpy = jest.fn();
        const previousClickSpy = jest.fn();
        const historyClickSpy = jest.fn();

        const wrapper = shallowMount<PeriodSelection>(PeriodSelection, {
            localVue,
            store,
        });

        wrapper.vm.onCurrentPeriodClick = currentClickSpy;
        wrapper.vm.onPreviousPeriodClick = previousClickSpy;
        wrapper.vm.redirect = historyClickSpy;

        await wrapper.find('.period-selection').trigger('click');
        await wrapper.findAll('.period-selection__dropdown__item').at(0).trigger('click');

        expect(currentClickSpy).toHaveBeenCalledTimes(1);

        await wrapper.find('.period-selection').trigger('click');
        await wrapper.findAll('.period-selection__dropdown__item').at(1).trigger('click');

        expect(previousClickSpy).toHaveBeenCalledTimes(1);

        await wrapper.find('.period-selection').trigger('click');
        await wrapper.find('.period-selection__dropdown__link-container').trigger('click');

        expect(historyClickSpy).toHaveBeenCalledTimes(1);
    });
});
