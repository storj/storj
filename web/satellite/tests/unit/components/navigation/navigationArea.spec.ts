// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Router from 'vue-router';
import Vuex from 'vuex';

import NavigationArea from '@/components/navigation/NavigationArea.vue';
import OnboardingTourArea from '@/components/onboardingTour/OnboardingTourArea.vue';
import ProjectDashboard from '@/components/project/ProjectDashboard.vue';

import { RouteConfig } from '@/router';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { NavigationLink } from '@/types/navigation';
import { Project } from '@/types/projects';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../../mock/api/projects';

const api = new ProjectsApiMock();
api.setMockProjects([new Project('1')]);
const projectsModule = makeProjectsModule(api);
const localVue = createLocalVue();

localVue.use(Vuex);
localVue.use(Router);

const store = new Vuex.Store({ modules: { projectsModule } });

const expectedLinks: NavigationLink[] = [
    RouteConfig.ProjectDashboard,
    RouteConfig.Team,
    RouteConfig.ApiKeys,
];

describe('NavigationArea', () => {
    it('snapshot not changed without project', (): void => {
        const router = new Router({
            mode: 'history',
            routes: [{
                path: '/',
                name: RouteConfig.OnboardingTour.name,
                component: OnboardingTourArea,
            }],
        });
        const wrapper = shallowMount(NavigationArea, {
            store,
            localVue,
            router,
        });

        const navigationElements = wrapper.findAll('.navigation-area__item-container');
        const disabledElements = wrapper.findAll('.navigation-area__item-container.disabled');
        const resourcesButton = wrapper.findAll('.navigation-area__resources-title__button');
        const accountButton = wrapper.findAll('.navigation-area__account-title__button');

        expect(navigationElements.length).toBe(7);
        expect(disabledElements.length).toBe(7);
        expect(resourcesButton.length).toBe(0);
        expect(accountButton.length).toBe(0);

        expect(wrapper).toMatchSnapshot();
    });

    const router = new Router({
        mode: 'history',
        routes: [{
            path: '/',
            name: RouteConfig.ProjectDashboard.name,
            component: ProjectDashboard,
        }],
    });

    it('snapshot not changed with project', async () => {
        const projects = await store.dispatch('fetchProjects');
        store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, projects[0].id);

        const wrapper = shallowMount(NavigationArea, {
            store,
            localVue,
            router,
        });

        const navigationElements = wrapper.findAll('.navigation-area__item-container');
        const disabledElements = wrapper.findAll('.navigation-area__item-container.disabled');
        const resourcesButton = wrapper.findAll('.navigation-area__resources-title__button');
        const accountButton = wrapper.findAll('.navigation-area__account-title__button');

        expect(navigationElements.length).toBe(7);
        expect(disabledElements.length).toBe(0);
        expect(resourcesButton.length).toBe(0);
        expect(accountButton.length).toBe(0);

        expect(wrapper).toMatchSnapshot();
    });

    it('navigation links are correct', () => {
        const wrapper = shallowMount(NavigationArea, {
            store,
            localVue,
            router,
        });

        const navigationLinks = (wrapper.vm as any).navigation;

        expect(navigationLinks.length).toBe(expectedLinks.length);

        expectedLinks.forEach((link, i) => {
            expect(navigationLinks[i].name).toBe(expectedLinks[i].name);
            expect(navigationLinks[i].path).toBe(expectedLinks[i].path);
        });
    });

    it('trigger show/hide events works correctly', async () => {
        const wrapper = shallowMount(NavigationArea, {
            store,
            localVue,
            router,
        });

        await wrapper.find('.navigation-area__resources-title').trigger('mouseenter');
        await wrapper.find('.navigation-area__account-title').trigger('mouseenter');

        expect(wrapper.find('.navigation-area__resources-title__button').text()).toMatch('Hide');
        await wrapper.find('.navigation-area__resources-title__button').trigger('click');

        expect(wrapper.find('.navigation-area__account-title__button').text()).toMatch('Hide');
        await wrapper.find('.navigation-area__account-title__button').trigger('click');

        expect(wrapper.find('.navigation-area__resources-title__button').text()).toMatch('Show');
        expect(wrapper.find('.navigation-area__account-title__button').text()).toMatch('Show');

        expect(wrapper.findAll('.navigation-area__item-container').length).toBe(3);

        await wrapper.find('.navigation-area__resources-title__button').trigger('click');
        await wrapper.find('.navigation-area__account-title__button').trigger('click');

        expect(wrapper.findAll('.navigation-area__item-container').length).toBe(7);

        await wrapper.find('.navigation-area__resources-title').trigger('mouseleave');
        await wrapper.find('.navigation-area__account-title').trigger('mouseleave');

        expect(wrapper.findAll('.navigation-area__resources-title__button').length).toBe(0);
        expect(wrapper.findAll('.navigation-area__account-title__button').length).toBe(0);

        expect(wrapper.findAll('.navigation-area__item-container').length).toBe(7);
    });
});
