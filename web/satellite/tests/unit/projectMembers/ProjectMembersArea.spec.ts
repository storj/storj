// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import ProjectMembersArea from '@/components/team/ProjectMembersArea.vue';

import { ProjectMembersApiGql } from '@/api/projectMembers';
import { appStateModule } from '@/store/modules/appState';
import { makeProjectMembersModule, PROJECT_MEMBER_MUTATIONS } from '@/store/modules/projectMembers';
import { ProjectMember, ProjectMembersPage } from '@/types/projectMembers';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();

localVue.use(Vuex);

const projectMember1 = new ProjectMember('testFullName1', 'testShortName1', 'test1@example.com', 'now1', '1');
const projectMember2 = new ProjectMember('testFullName2', 'testShortName2', 'test2@example.com', 'now2', '2');

describe('ProjectMembersArea.vue', () => {
    const pmApi = new ProjectMembersApiGql();
    const projectMembersModule = makeProjectMembersModule(pmApi);

    const store = new Vuex.Store({modules: { projectMembersModule, appStateModule }});

    it('renders correctly', () => {
        const wrapper = shallowMount(ProjectMembersArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('empty search result area render correctly', function () {
        const wrapper = shallowMount(ProjectMembersArea, {
            store,
            localVue,
        });

        const emptySearchResultArea = wrapper.findAll('.team-area__empty-search-result-area');
        expect(emptySearchResultArea.length).toBe(1);

        const teamContainer = wrapper.findAll('.team-area__container');
        expect(teamContainer.length).toBe(0);

        expect(wrapper).toMatchSnapshot();
    });

    it('function fetchProjectMembers works correctly', () => {
        const testProjectMembersPage = new ProjectMembersPage();
        testProjectMembersPage.projectMembers = [projectMember1];
        testProjectMembersPage.totalCount = 1;
        testProjectMembersPage.pageCount = 1;

        store.commit(PROJECT_MEMBER_MUTATIONS.FETCH, testProjectMembersPage);

        const wrapper = shallowMount(ProjectMembersArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.projectMembers.length).toBe(1);
    });

    it('team area renders correctly', function () {
        const wrapper = shallowMount(ProjectMembersArea, {
            store,
            localVue,
        });

        const emptySearchResultArea = wrapper.findAll('.team-area__empty-search-result-area');
        expect(emptySearchResultArea.length).toBe(0);

        const teamContainer = wrapper.findAll('.team-area__container');
        expect(teamContainer.length).toBe(1);

        const sortingListHeaderStub = wrapper.findAll('sortinglistheader-stub');
        expect(sortingListHeaderStub.length).toBe(1);

        const listStub = wrapper.findAll('vlist-stub');
        expect(listStub.length).toBe(1);

        expect(wrapper).toMatchSnapshot();
    });

    it('action on toggle works correctly', () => {
        const wrapper = shallowMount(ProjectMembersArea, {
            store,
            localVue,
        });

        wrapper.vm.onMemberClick(projectMember1);

        expect(store.getters.selectedProjectMembers.length).toBe(1);
    });

    it('clear selection works correctly', () => {
        const wrapper = shallowMount(ProjectMembersArea, {
            store,
            localVue,
        });

        wrapper.vm.onMemberClick(projectMember1);

        expect(store.getters.selectedProjectMembers.length).toBe(0);
    });

    it('Reversing list order triggers rerender', () => {
        const testProjectMembersPage = new ProjectMembersPage();
        testProjectMembersPage.projectMembers = [projectMember1, projectMember2];
        testProjectMembersPage.totalCount = 2;
        testProjectMembersPage.pageCount = 1;

        store.commit(PROJECT_MEMBER_MUTATIONS.FETCH, testProjectMembersPage);

        const wrapper = shallowMount(ProjectMembersArea, {
            store,
            localVue,
        });

        expect(wrapper.vm.projectMembers[0].user.id).toBe(projectMember1.user.id);

        testProjectMembersPage.projectMembers = [projectMember2, projectMember1];

        store.commit(PROJECT_MEMBER_MUTATIONS.FETCH, testProjectMembersPage);

        expect(wrapper.vm.projectMembers[0].user.id).toBe(projectMember2.user.id);
    });
});
