// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue } from '@vue/test-utils';
import { PROJECT_MEMBER_MUTATIONS } from '@/store/mutationConstants';
import Vuex from 'vuex';
import { ProjectMember, ProjectMemberCursor, ProjectMemberOrderBy, ProjectMembersPage } from '@/types/projectMembers';
import { ProjectMembersApiGql } from '@/api/projectMembers';
import { makeProjectMembersModule } from '@/store/modules/projectMembers';
import { SortDirection } from '@/types/common';

const Vue = createLocalVue();
const pmApi = new ProjectMembersApiGql();
const projectMembersModule = makeProjectMembersModule(pmApi);

Vue.use(Vuex);

const store = new Vuex.Store({modules: {projectMembersModule}});
const state = (store.state as any).projectMembersModule;

const projectMember1 = new ProjectMember('testFullName1', 'testShortName1', 'test1@example.com', 'now1', '1');
const projectMember2 = new ProjectMember('testFullName2', 'testShortName2', 'test2@example.com', 'now2', '2');

describe('mutations', () => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
    });

    it('fetch project members', function () {
        const testProjectMembersPage = new ProjectMembersPage();
        testProjectMembersPage.projectMembers = [projectMember1];
        testProjectMembersPage.totalCount = 1;
        testProjectMembersPage.pageCount = 1;

        store.commit(PROJECT_MEMBER_MUTATIONS.FETCH, testProjectMembersPage);

        expect(state.page.projectMembers.length).toBe(1);
        expect(state.page.search).toBe('');
        expect(state.page.order).toBe(ProjectMemberOrderBy.NAME);
        expect(state.page.orderDirection).toBe(SortDirection.ASCENDING);
        expect(state.page.limit).toBe(6);
        expect(state.page.pageCount).toBe(1);
        expect(state.page.currentPage).toBe(1);
        expect(state.page.totalCount).toBe(1);
    });

    it('set project members page', function () {
        store.commit(PROJECT_MEMBER_MUTATIONS.SET_PAGE, 2);

        expect(state.cursor.page).toBe(2);
    });

    it('set search query', function () {
        store.commit(PROJECT_MEMBER_MUTATIONS.SET_SEARCH_QUERY, 'testSearchQuery');

        expect(state.cursor.search).toBe('testSearchQuery');
    });

    it('set sort order', function () {
        store.commit(PROJECT_MEMBER_MUTATIONS.CHANGE_SORT_ORDER, ProjectMemberOrderBy.EMAIL);

        expect(state.cursor.order).toBe(ProjectMemberOrderBy.EMAIL);
    });

    it('set sort direction', function () {
        store.commit(PROJECT_MEMBER_MUTATIONS.CHANGE_SORT_ORDER_DIRECTION, SortDirection.DESCENDING);

        expect(state.cursor.orderDirection).toBe(SortDirection.DESCENDING);
    });

    it('set search query', function () {
        store.commit(PROJECT_MEMBER_MUTATIONS.TOGGLE_SELECTION, projectMember1.user.id);

        expect(state.page.projectMembers[0].isSelected).toBe(true);
    });

    it('clear selection', function () {
        store.commit(PROJECT_MEMBER_MUTATIONS.CLEAR_SELECTION);

        expect(state.page.projectMembers[0].isSelected).toBe(false);
    });


});

// describe('actions', () => {
//     beforeEach(() => {
//         jest.resetAllMocks();
//     });
//
//     it('success add project members', async function () {
//         const rootGetters = {
//             selectedProject: {
//                 id: '1'
//             },
//             searchParameters: {},
//             pagination: {limit: 20, offset: 0}
//         };
//         jest.spyOn(api, 'addProjectMembersRequest').mockReturnValue(Promise.resolve(<RequestResponse<null>>{isSuccess: true}));
//
//         const emails = ['1', '2'];
//
//         const dispatchResponse = await projectMembersModule.actions.addProjectMembers({rootGetters}, emails);
//
//         expect(dispatchResponse.isSuccess).toBeTruthy();
//     });
//
//     it('error add project members', async function () {
//         const rootGetters = {
//             selectedProject: {
//                 id: '1'
//             }
//         };
//         jest.spyOn(api, 'addProjectMembersRequest').mockReturnValue(Promise.resolve(<RequestResponse<null>>{isSuccess: false}));
//
//         const emails = ['1', '2'];
//
//         const dispatchResponse = await projectMembersModule.actions.addProjectMembers({rootGetters}, emails);
//
//         expect(dispatchResponse.isSuccess).toBeFalsy();
//     });
//
//     it('success delete project members', async () => {
//         const rootGetters = {
//             selectedProject: {
//                 id: '1'
//             }
//         };
//         jest.spyOn(api, 'deleteProjectMembersRequest').mockReturnValue(Promise.resolve(<RequestResponse<null>>{isSuccess: true}));
//
//         const commit = jest.fn();
//         const emails = ['1', '2'];
//
//         const dispatchResponse = await projectMembersModule.actions.deleteProjectMembers({commit, rootGetters}, emails);
//
//         expect(dispatchResponse.isSuccess).toBeTruthy();
//         expect(commit).toHaveBeenCalledWith(PROJECT_MEMBER_MUTATIONS.DELETE, emails);
//     });
//
//     it('error delete project members', async () => {
//         const rootGetters = {
//             selectedProject: {
//                 id: '1'
//             }
//         };
//         jest.spyOn(api, 'deleteProjectMembersRequest').mockReturnValue(Promise.resolve(<RequestResponse<null>>{isSuccess: false}));
//
//         const commit = jest.fn();
//         const emails = ['1', '2'];
//
//         const dispatchResponse = await projectMembersModule.actions.deleteProjectMembers({commit, rootGetters}, emails);
//
//         expect(dispatchResponse.isSuccess).toBeFalsy();
//         expect(commit).toHaveBeenCalledTimes(0);
//     });
//
//     it('toggle selection', function () {
//         const commit = jest.fn();
//         const projectMemberId = '1';
//
//         projectMembersModule.actions.toggleProjectMemberSelection({commit}, projectMemberId);
//
//         expect(commit).toHaveBeenCalledWith(PROJECT_MEMBER_MUTATIONS.TOGGLE_SELECTION, projectMemberId);
//     });
//
//     it('clear selection', function () {
//         const commit = jest.fn();
//
//         projectMembersModule.actions.clearProjectMemberSelection({commit});
//
//         expect(commit).toHaveBeenCalledTimes(1);
//     });
//
//     it('success fetch project members', async function () {
//         const rootGetters = {
//             selectedProject: {
//                 id: '1'
//             }
//         };
//         const state = {
//             pagination: {
//                 limit: 20,
//                 offset: 0
//             },
//             searchParameters: {
//                 searchQuery: ''
//             }
//         };
//         const commit = jest.fn();
//         const projectMemberMockModel: ProjectMember = new ProjectMember('1', '1', '1', '1', '1');
//         jest.spyOn(api, 'fetchProjectMembersRequest').mockReturnValue(
//             Promise.resolve(<RequestResponse<ProjectMember[]>>{
//                 isSuccess: true,
//                 data: [projectMemberMockModel]
//             }));
//
//         const dispatchResponse = await projectMembersModule.actions.fetchProjectMembers({
//             state,
//             commit,
//             rootGetters
//         });
//
//         expect(dispatchResponse.isSuccess).toBeTruthy();
//         expect(commit).toHaveBeenCalledWith(PROJECT_MEMBER_MUTATIONS.FETCH, [projectMemberMockModel]);
//     });
//
//     it('error fetch project members', async function () {
//         const rootGetters = {
//             selectedProject: {
//                 id: '1'
//             }
//         };
//         const state = {
//             pagination: {
//                 limit: 20,
//                 offset: 0
//             },
//             searchParameters: {
//                 searchQuery: ''
//             }
//         };
//         const commit = jest.fn();
//         jest.spyOn(api, 'fetchProjectMembersRequest').mockReturnValue(
//             Promise.resolve(<RequestResponse<ProjectMember[]>>{
//                 isSuccess: false,
//             })
//         );
//
//         const dispatchResponse = await projectMembersModule.actions.fetchProjectMembers({
//             state,
//             commit,
//             rootGetters
//         });
//
//         expect(dispatchResponse.isSuccess).toBeFalsy();
//         expect(commit).toHaveBeenCalledTimes(0);
//     });
// });

// describe('getters', () => {
//     it('project members', function () {
//         const state = {
//             projectMembers: [{user: {email: '1'}}]
//         };
//         const retrievedProjectMembers = projectMembersModule.getters.projectMembers(state);
//
//         expect(retrievedProjectMembers.length).toBe(1);
//     });
//
//     it('selected project members', function () {
//         const state = {
//             projectMembers: [
//                 {isSelected: false},
//                 {isSelected: true},
//                 {isSelected: true},
//                 {isSelected: true},
//                 {isSelected: false},
//             ]
//         };
//         const retrievedSelectedProjectMembers = projectMembersModule.getters.selectedProjectMembers(state);
//         expect(retrievedSelectedProjectMembers.length).toBe(3);
//     });
// });
