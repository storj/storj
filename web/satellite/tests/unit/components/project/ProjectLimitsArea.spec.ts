// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import ProjectLimitsArea from '@/components/project/ProjectLimitsArea.vue';

import { makeProjectsModule } from '@/store/modules/projects';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../../mock/api/projects';

const localVue = createLocalVue();
localVue.use(Vuex);

const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const store = new Vuex.Store({ modules: { projectsModule }});

describe('ProjectLimitsArea', () => {
    it('snapshot not changed', () => {
        const wrapper = shallowMount(ProjectLimitsArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
