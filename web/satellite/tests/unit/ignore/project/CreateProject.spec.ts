// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { shallowMount } from '@vue/test-utils';

import CreateProject from '@/components/project/CreateProject.vue';

describe('CreateProject.vue', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(CreateProject);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with project name', async (): Promise<void> => {
        const wrapper = shallowMount(CreateProject);

        await wrapper.vm.setProjectName('testName');

        expect(wrapper.findAll('.disabled').length).toBe(0);
    });
});
