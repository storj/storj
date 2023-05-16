// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '@/../tests/unit/mock/api/projects';
import { ProjectMember, ProjectMembersPage } from '@/types/projectMembers';
import { Project } from '@/types/projects';

import ProjectMembersArea from '@/components/team/ProjectMembersArea.vue';

const projectsApi = new ProjectsApiMock();

describe('ProjectMembersArea.vue', () => {
    const project = new Project('id', 'projectName', 'projectDescription', 'test', 'testOwnerId', true);
    projectsApi.setMockProjects([project]);
    const date = new Date(0);
    const projectMember1 = new ProjectMember('testFullName1', 'testShortName1', 'test1@example.com', date, '1');
    const projectMember2 = new ProjectMember('testFullName2', 'testShortName2', 'test2@example.com', date, '2');

    const testProjectMembersPage = new ProjectMembersPage();
    testProjectMembersPage.projectMembers = [projectMember1];
    testProjectMembersPage.totalCount = 1;
    testProjectMembersPage.pageCount = 1;

    // pmApi.setMockPage(testProjectMembersPage);

    it('renders correctly', async (): Promise<void> => {
        const wrapper = shallowMount(ProjectMembersArea);

        await wrapper.setData({ areMembersFetching: false });

        expect(wrapper).toMatchSnapshot();
    });

    it('function fetchProjectMembers works correctly', () => {
        // store.commit(FETCH, testProjectMembersPage);

        const wrapper = shallowMount(ProjectMembersArea);

        expect(wrapper.vm.projectMembers).toEqual([projectMember1]);
    });

    it('team area renders correctly', async (): Promise<void> => {
        // store.commit(FETCH, testProjectMembersPage);

        const wrapper = shallowMount(ProjectMembersArea);

        await wrapper.setData({ areMembersFetching: false });

        const emptySearchResultArea = wrapper.findAll('.team-area__empty-search-result-area');
        expect(emptySearchResultArea.length).toBe(0);

        const teamContainer = wrapper.findAll('.team-area__table');
        expect(teamContainer.length).toBe(1);

        expect(wrapper).toMatchSnapshot();
    });

    it('action on toggle works correctly', () => {
        // store.commit(FETCH, testProjectMembersPage);

        const wrapper = shallowMount(ProjectMembersArea);

        wrapper.vm.onMemberCheckChange(projectMember1);

        // expect(store.getters.selectedProjectMembers.length).toBe(1);
    });

    it('clear selection works correctly', () => {
        const date = new Date(0);
        const projectMember3 = new ProjectMember('testFullName1', 'testShortName1', 'test1@example.com', date, '1');
        projectMember3.isSelected = true;
        const testProjectMembersPage = new ProjectMembersPage();
        testProjectMembersPage.projectMembers = [projectMember3];
        testProjectMembersPage.totalCount = 1;
        testProjectMembersPage.pageCount = 1;
        // store.commit(FETCH, testProjectMembersPage);

        const wrapper = shallowMount(ProjectMembersArea);

        wrapper.vm.onMemberCheckChange(projectMember3);

        // expect(store.getters.selectedProjectMembers.length).toBe(0);
    });

    it('Reversing list order triggers rerender', () => {
        const testPage = new ProjectMembersPage();
        testPage.projectMembers = [projectMember1, projectMember2];
        testPage.totalCount = 2;
        testPage.pageCount = 1;
        // pmApi.setMockPage(testPage);

        // store.commit(FETCH, testPage);

        const wrapper = shallowMount(ProjectMembersArea);

        // expect(wrapper.vm.projectMembers[0].user.id).toBe(projectMember1.user.id);

        testProjectMembersPage.projectMembers = [projectMember2, projectMember1];

        // store.commit(FETCH, testProjectMembersPage);

        expect(wrapper.vm.projectMembers[0].user.id).toBe(projectMember2.user.id);
    });

    it('empty search result area render correctly', async (): Promise<void> => {
        const testPage1 = new ProjectMembersPage();
        testPage1.projectMembers = [];
        testPage1.totalCount = 0;
        testPage1.pageCount = 0;
        testPage1.search = 'testSearch';
        // pmApi.setMockPage(testPage1);

        // store.commit(FETCH, testPage1);

        const wrapper = shallowMount(ProjectMembersArea);

        await wrapper.setData({ areMembersFetching: false });

        const emptySearchResultArea = wrapper.findAll('.team-area__empty-search-result-area');
        expect(emptySearchResultArea.length).toBe(1);

        const teamContainer = wrapper.findAll('.team-area__container');
        expect(teamContainer.length).toBe(0);

        expect(wrapper).toMatchSnapshot();
    });
});
