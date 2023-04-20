// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectMember } from '@/types/projectMembers';

import ProjectMemberListItem from '@/components/team/ProjectMemberListItem.vue';

const localVue = createLocalVue();

describe('', () => {
    const data = new Date(0);
    const member: ProjectMember = new ProjectMember('testFullName', 'testShortName', 'test@example.com', data, '1');

    it('should render correctly', function () {
        const wrapper = shallowMount(ProjectMemberListItem, {
            localVue,
            propsData: {
                itemData: member,
            },
        });

        expect(wrapper).toMatchSnapshot();
        // expect(store.getters.selectedProject.ownerId).toBe(member.user.id);
        expect(wrapper.findAll('.owner').length).toBe(1);
    });

    it('should render correctly with item row highlighted', function () {
        member.isSelected = true;

        const wrapper = shallowMount(ProjectMemberListItem, {
            localVue,
            propsData: {
                itemData: member,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
