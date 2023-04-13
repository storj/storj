// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, shallowMount } from '@vue/test-utils';

import { NotificatorPlugin } from '@/utils/plugins/notificator';

import CreateProject from '@/components/project/CreateProject.vue';

const localVue = createLocalVue();
localVue.use(new NotificatorPlugin());

describe('CreateProject.vue', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount<CreateProject>(CreateProject, {
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with project name', async (): Promise<void> => {
        const wrapper = shallowMount<CreateProject>(CreateProject, {
            localVue,
        });

        await wrapper.vm.setProjectName('testName');

        expect(wrapper.findAll('.disabled').length).toBe(0);
    });
});
