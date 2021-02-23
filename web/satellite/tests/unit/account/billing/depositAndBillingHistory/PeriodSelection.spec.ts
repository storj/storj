// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';
import { VNode } from 'vue';
import { DirectiveBinding } from 'vue/types/options';
import Vuex from 'vuex';

import PeriodSelection from '@/components/account/billing/depositAndBillingHistory/PeriodSelection.vue';

import { appStateModule } from '@/store/modules/appState';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { Project } from '@/types/projects';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../../../mock/api/projects';

const localVue = createLocalVue();
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const project = new Project('id', 'projectName', 'projectDescription', 'test', 'testOwnerId', false);

let clickOutsideEvent: EventListener;

localVue.directive('click-outside', {
    bind: function (el: HTMLElement, binding: DirectiveBinding, vnode: VNode) {
        clickOutsideEvent = function(event: Event): void {
            if (el === event.target) {
                return;
            }

            if (vnode.context) {
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

const store = new Vuex.Store({ modules: { projectsModule, appStateModule }});
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
        const wrapper = mount(PeriodSelection, {
            localVue,
            store,
        });

        await wrapper.find('.period-selection').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });

    it('clicks work correctly', async (): Promise<void> => {
        const currentClickSpy = sinon.spy();
        const previousClickSpy = sinon.spy();
        const historyClickSpy = sinon.spy();
        const wrapper = mount(PeriodSelection, {
            localVue,
            store,
        });

        wrapper.vm.onCurrentPeriodClick = currentClickSpy;
        wrapper.vm.onPreviousPeriodClick = previousClickSpy;
        wrapper.vm.redirect = historyClickSpy;

        await wrapper.find('.period-selection').trigger('click');
        await wrapper.findAll('.period-selection__dropdown__item').at(0).trigger('click');

        expect(currentClickSpy.callCount).toBe(1);

        await wrapper.find('.period-selection').trigger('click');
        await wrapper.findAll('.period-selection__dropdown__item').at(1).trigger('click');

        expect(previousClickSpy.callCount).toBe(1);

        await wrapper.find('.period-selection').trigger('click');
        await wrapper.find('.period-selection__dropdown__link-container').trigger('click');

        expect(historyClickSpy.callCount).toBe(1);
    });
});
