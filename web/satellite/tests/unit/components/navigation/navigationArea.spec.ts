// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import NavigationArea from '@/components/navigation/NavigationArea.vue';

import { RouteConfig } from '@/router';
import { makeProjectsModule } from '@/store/modules/projects';
import { NavigationLink } from '@/types/navigation';
import { Project } from '@/types/projects';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../../mock/api/projects';

const api = new ProjectsApiMock();
api.setMockProject(new Project('1'));
const projectsModule = makeProjectsModule(api);
const localVue = createLocalVue();

localVue.use(Vuex);

const store = new Vuex.Store({ modules: { projectsModule } });

const expectedLinks: NavigationLink[] = [
    RouteConfig.ProjectOverview,
    RouteConfig.Team,
    RouteConfig.ApiKeys,
    RouteConfig.Buckets,
];

describe('NavigationArea', () => {
    it('snapshot not changed without project', () => {
        const wrapper = shallowMount(NavigationArea, {
            store,
            localVue,
        });

        const navigationElements = wrapper.findAll('.navigation-area__item-container');
        const disabledElements = wrapper.findAll('.navigation-area__item-container.disabled');
        const resourcesButton = wrapper.findAll('.navigation-area__resources-title__button');
        const accountButton = wrapper.findAll('.navigation-area__account-title__button');

        expect(navigationElements.length).toBe(4);
        expect(disabledElements.length).toBe(4);
        expect(resourcesButton.length).toBe(0);
        expect(accountButton.length).toBe(0);

        expect(wrapper).toMatchSnapshot();
    });

    it('snapshot not changed with project', async () => {
        const projects = await store.dispatch('fetchProjects');
        await store.dispatch('selectProject', projects[0].id);

        const wrapper = shallowMount(NavigationArea, {
            store,
            localVue,
        });

        const navigationElements = wrapper.findAll('.navigation-area__item-container');
        const disabledElements = wrapper.findAll('.navigation-area__item-container.disabled');
        const resourcesButton = wrapper.findAll('.navigation-area__resources-title__button');
        const accountButton = wrapper.findAll('.navigation-area__account-title__button');

        expect(navigationElements.length).toBe(4);
        expect(disabledElements.length).toBe(0);
        expect(resourcesButton.length).toBe(0);
        expect(accountButton.length).toBe(0);

        expect(wrapper).toMatchSnapshot();
    });

    it('navigation links are correct', () => {
        const wrapper = shallowMount(NavigationArea, {
            store,
            localVue,
        });

        const navigationLinks = (wrapper.vm as any).navigation;

        expect(navigationLinks.length).toBe(expectedLinks.length);

        expectedLinks.forEach((link, i) => {
            expect(navigationLinks[i].name).toBe(expectedLinks[i].name);
            expect(navigationLinks[i].path).toBe(expectedLinks[i].path);
        });
    });

    it('trigger show/hide events works correctly', () => {
        const wrapper = shallowMount(NavigationArea, {
            store,
            localVue,
        });

        wrapper.find('.navigation-area__resources-title').trigger('mouseenter');
        wrapper.find('.navigation-area__account-title').trigger('mouseenter');

        expect(wrapper.find('.navigation-area__resources-title__button').text()).toMatch('Show');
        wrapper.find('.navigation-area__resources-title__button').trigger('click');

        expect(wrapper.find('.navigation-area__account-title__button').text()).toMatch('Show');
        wrapper.find('.navigation-area__account-title__button').trigger('click');

        expect(wrapper.find('.navigation-area__resources-title__button').text()).toMatch('Hide');
        expect(wrapper.find('.navigation-area__account-title__button').text()).toMatch('Hide');

        expect(wrapper.findAll('.navigation-area__item-container').length).toBe(9);

        wrapper.find('.navigation-area__resources-title__button').trigger('click');
        wrapper.find('.navigation-area__account-title__button').trigger('click');

        expect(wrapper.findAll('.navigation-area__item-container').length).toBe(4);

        wrapper.find('.navigation-area__resources-title').trigger('mouseleave');
        wrapper.find('.navigation-area__account-title').trigger('mouseleave');

        expect(wrapper.findAll('.navigation-area__resources-title__button').length).toBe(0);
        expect(wrapper.findAll('.navigation-area__account-title__button').length).toBe(0);

        expect(wrapper.findAll('.navigation-area__item-container').length).toBe(4);
    });
});
