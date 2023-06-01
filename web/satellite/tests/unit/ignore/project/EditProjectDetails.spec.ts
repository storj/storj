// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '@/../tests/unit/mock/api/projects';
import { ProjectLimits } from '@/types/projects';

import EditProjectDetails from '@/components/project/EditProjectDetails.vue';

const projectLimits = new ProjectLimits(1000, 100, 1000, 100);
const projectsApi = new ProjectsApiMock();
projectsApi.setMockLimits(projectLimits);

describe('EditProjectDetails.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(EditProjectDetails);

        expect(wrapper).toMatchSnapshot();
    });

    it('editing name works correctly', async (): Promise<void> => {
        const wrapper = shallowMount(EditProjectDetails);

        await wrapper.vm.toggleNameEditing();

        expect(wrapper).toMatchSnapshot();

        const newName = 'new name';

        wrapper.vm.$data.nameValue = newName;
        await wrapper.vm.onSaveNameButtonClick();

        expect(wrapper).toMatchSnapshot();
        // await expect(store.getters.selectedProject.name).toMatch(newName);
    });

    it('editing description works correctly', async (): Promise<void> => {
        const wrapper = shallowMount(EditProjectDetails);

        await wrapper.vm.toggleDescriptionEditing();

        expect(wrapper).toMatchSnapshot();

        const newDescription = 'new description';

        wrapper.vm.$data.descriptionValue = newDescription;
        await wrapper.vm.onSaveDescriptionButtonClick();

        expect(wrapper).toMatchSnapshot();
        // await expect(store.getters.selectedProject.description).toMatch(newDescription);
    });
});
