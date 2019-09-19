// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import ProjectMemberListItem from '@/components/team/ProjectMemberListItem.vue';

import { ProjectMember } from '@/types/projectMembers';
import { mount } from '@vue/test-utils';

describe('', () => {
    it('should renders correctly', function () {
        const member: ProjectMember = new ProjectMember('testFullName', 'testShortName', 'test@example.com', '2019-08-09T08:52:43.695679Z', '1');

        const wrapper = mount(ProjectMemberListItem, {
            propsData: {
                itemData: member,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('should render correctly with item row highlighted', function () {
        const member: ProjectMember = new ProjectMember('testFullName', 'testShortName', 'test@example.com', '2019-08-09T08:52:43.695679Z', '1');
        member.isSelected = true;

        const wrapper = mount(ProjectMemberListItem, {
            propsData: {
                itemData: member,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
