// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { PROJECT_MEMBER_MUTATIONS } from '../mutationConstants';
import {
    addProjectMembersRequest,
    deleteProjectMembersRequest,
    fetchProjectMembersRequest,
    fetchProjectMembersRequest1
} from '@/api/projectMembers';
import { ProjectMemberSortByEnum } from '@/utils/constants/ProjectMemberSortEnum';
import { PM_ACTIONS } from '@/utils/constants/actionNames';
import { ProjectMemberCursor, ProjectMembersPage, TeamMemberModel } from '@/types/projects';

export const projMembersModule = {
    state: {
        projectMembers: [],
        projectMembersCount: 0,
        searchParameters: {
            sortBy: ProjectMemberSortByEnum.NAME,
            searchQuery: ''
        },
        pagination: {
            offset: 0,
            limit: 20,
        }
    },
    mutations: {
        [PROJECT_MEMBER_MUTATIONS.DELETE](state: any, projectMemberEmails: string[]) {
            const emailsCount = projectMemberEmails.length;

            for (let j = 0; j < emailsCount; j++) {
                state.projectMembers = state.projectMembers.filter((element: any) => {
                    return element.user.email !== projectMemberEmails[j];
                });
            }
        },
        [PROJECT_MEMBER_MUTATIONS.TOGGLE_SELECTION](state: any, projectMemberId: string) {
            state.projectMembers = state.projectMembers.map((projectMember: any) => {
                if (projectMember.user.id === projectMemberId) {
                    projectMember.isSelected = !projectMember.isSelected;
                }

                return projectMember;
            });
        },
        [PROJECT_MEMBER_MUTATIONS.CLEAR_SELECTION](state: any) {
            state.projectMembers = state.projectMembers.map((projectMember: any) => {
                projectMember.isSelected = false;

                return projectMember;
            });
        },
        [PROJECT_MEMBER_MUTATIONS.FETCH](state: any, teamMembers: any[]) {
            teamMembers.forEach(value => {
                state.projectMembers.push(value);

            });
            state.projectMembersCount = state.projectMembers.length;
        },
        [PROJECT_MEMBER_MUTATIONS.CLEAR](state: any) {
            state.projectMembers = [];
        },
        [PROJECT_MEMBER_MUTATIONS.CHANGE_SORT_ORDER](state: any, sortBy: ProjectMemberSortByEnum) {
            state.searchParameters.sortBy = sortBy;
        },
        [PROJECT_MEMBER_MUTATIONS.SET_SEARCH_QUERY](state: any, searchQuery: string) {
            state.searchParameters.searchQuery = searchQuery;
        },
        [PROJECT_MEMBER_MUTATIONS.ADD_OFFSET](state: any) {
            state.pagination.offset += state.pagination.limit;
        },
        [PROJECT_MEMBER_MUTATIONS.CLEAR_OFFSET](state: any) {
            state.pagination.offset = 0;
        }

    },
    actions: {
        addProjectMembers: async function ({rootGetters}: any, emails: string[]): Promise<RequestResponse<null>> {
            const projectId = rootGetters.selectedProject.id;

            return await addProjectMembersRequest(projectId, emails);
        },
        deleteProjectMembers: async function ({commit, rootGetters}: any, projectMemberEmails: string[]): Promise<RequestResponse<null>> {
            const projectId = rootGetters.selectedProject.id;

            const response = await deleteProjectMembersRequest(projectId, projectMemberEmails);

            if (response.isSuccess) {
                commit(PROJECT_MEMBER_MUTATIONS.DELETE, projectMemberEmails);
            }

            return response;
        },
        toggleProjectMemberSelection: function ({commit}: any, projectMemberId: string) {
            commit(PROJECT_MEMBER_MUTATIONS.TOGGLE_SELECTION, projectMemberId);
        },
        clearProjectMemberSelection: function ({commit}: any) {
            commit(PROJECT_MEMBER_MUTATIONS.CLEAR_SELECTION);
        },
        fetchProjectMembers: async function ({commit, state, rootGetters}: any): Promise<RequestResponse<TeamMemberModel[]>> {
            const projectId = rootGetters.selectedProject.id;
            const response: RequestResponse<TeamMemberModel[]> = await fetchProjectMembersRequest(projectId, state.pagination.limit, state.pagination.offset,
                state.searchParameters.sortBy, state.searchParameters.searchQuery);

            if (response.isSuccess) {
                commit(PROJECT_MEMBER_MUTATIONS.FETCH, response.data);

                if (response.data.length > 0) {
                    commit(PROJECT_MEMBER_MUTATIONS.ADD_OFFSET);
                }
            }

            return response;
        },
        setProjectMembersSortingBy: function ({commit, dispatch}, sortBy: ProjectMemberSortByEnum) {
            commit(PROJECT_MEMBER_MUTATIONS.CHANGE_SORT_ORDER, sortBy);
            commit(PROJECT_MEMBER_MUTATIONS.CLEAR);
            commit(PROJECT_MEMBER_MUTATIONS.CLEAR_OFFSET);
        },
        setProjectMembersSearchQuery: function ({commit, dispatch}, searchQuery: string) {
            commit(PROJECT_MEMBER_MUTATIONS.SET_SEARCH_QUERY, searchQuery);
            commit(PROJECT_MEMBER_MUTATIONS.CLEAR);
            commit(PROJECT_MEMBER_MUTATIONS.CLEAR_OFFSET);
        },
        clearProjectMembers: function ({commit}: any) {
            commit(PROJECT_MEMBER_MUTATIONS.CLEAR);
        },
        clearProjectMembersOffset: function ({commit}) {
            commit(PROJECT_MEMBER_MUTATIONS.CLEAR_OFFSET);
        }
    },
    getters: {
        projectMembers: (state: any) => state.projectMembers,
        projectMembersCountGetter: (state: any) => state.projectMembersCount,
        selectedProjectMembers: (state: any) => state.projectMembers.filter((member: any) => member.isSelected),
    },
};

const projectMembersLimit = 8;
const firstPage = 1;
export const projectMembersModule = {
    state: {
        cursor: {
            order: ProjectMemberSortByEnum.NAME,
            limit: projectMembersLimit,
            search: '',
            page: firstPage
        } as ProjectMemberCursor,
        page: {projectMembers: [] as TeamMemberModel[]} as ProjectMembersPage,
    },
    mutations: {
        [PROJECT_MEMBER_MUTATIONS.DELETE](state: any, projectMemberEmails: string[]) {
            const emailsCount = projectMemberEmails.length;

            for (let j = 0; j < emailsCount; j++) {
                state.page.projectMembers = state.page.projectMembers.filter((element: any) => {
                    return element.user.email !== projectMemberEmails[j];
                });
            }
        },
        [PROJECT_MEMBER_MUTATIONS.FETCH](state: any, page: ProjectMembersPage) {
            // todo expand this assignment
            console.log(page);
            state.page = page;
        },
        [PROJECT_MEMBER_MUTATIONS.SET_PAGE](state: any, page: number) {
            state.cursor.page = page;
        },
        [PROJECT_MEMBER_MUTATIONS.SET_SEARCH_QUERY](state: any, search: string) {
            state.cursor.search = search;
        },
        [PROJECT_MEMBER_MUTATIONS.CHANGE_SORT_ORDER](state: any, order: ProjectMemberSortByEnum) {
            state.cursor.order = order;
        },
        [PROJECT_MEMBER_MUTATIONS.CLEAR](state: any) {
            state.cursor = {limit: projectMembersLimit, search: '', page: firstPage} as ProjectMemberCursor;
            state.page = {projectMembers: [] as TeamMemberModel[]} as ProjectMembersPage;
        },
        [PROJECT_MEMBER_MUTATIONS.TOGGLE_SELECTION](state: any, projectMemberId: string) {
            state.page.projectMembers = state.page.projectMembers.map((projectMember: any) => {
                if (projectMember.user.id === projectMemberId) {
                    projectMember.isSelected = !projectMember.isSelected;
                }

                return projectMember;
            });
        },
        [PROJECT_MEMBER_MUTATIONS.CLEAR_SELECTION](state: any) {
            state.page.projectMembers = state.page.projectMembers.map((projectMember: any) => {
                projectMember.isSelected = false;

                return projectMember;
            });
        },
    },
    actions: {
        [PM_ACTIONS.ADD]: async function ({rootGetters}: any, emails: string[]): Promise<RequestResponse<null>> {
            const projectId = rootGetters.selectedProject.id;

            return await addProjectMembersRequest(projectId, emails);
        },
        [PM_ACTIONS.DELETE]: async function ({commit, rootGetters}: any, projectMemberEmails: string[]): Promise<RequestResponse<null>> {
            const projectId = rootGetters.selectedProject.id;

            const response = await deleteProjectMembersRequest(projectId, projectMemberEmails);

            if (response.isSuccess) {
                commit(PROJECT_MEMBER_MUTATIONS.DELETE, projectMemberEmails);
            }

            return response;
        },
        [PM_ACTIONS.FETCH]: async function ({commit, rootGetters, state}: any, page: number): Promise<RequestResponse<ProjectMembersPage>> {
            const projectID = rootGetters.selectedProject.id;
            state.cursor.page = page;

            commit(PROJECT_MEMBER_MUTATIONS.SET_PAGE, page);

            let result = await fetchProjectMembersRequest1(projectID, state.cursor);
            if (result.isSuccess) {
                commit(PROJECT_MEMBER_MUTATIONS.FETCH, result.data);
            }

            return result;
        },

        [PM_ACTIONS.SET_SEARCH_QUERY]: function ({commit}, search: string) {
            commit(PROJECT_MEMBER_MUTATIONS.SET_SEARCH_QUERY, search);
        },
        [PM_ACTIONS.SET_SORT_BY]: function ({commit}, order: ProjectMemberSortByEnum) {
            commit(PROJECT_MEMBER_MUTATIONS.CHANGE_SORT_ORDER, order);
        },
        [PM_ACTIONS.CLEAR]: function ({commit}) {
            commit(PROJECT_MEMBER_MUTATIONS.CLEAR);
        },
        [PM_ACTIONS.TOGGLE_SELECTION]: function ({commit}: any, projectMemberId: string) {
            commit(PROJECT_MEMBER_MUTATIONS.TOGGLE_SELECTION, projectMemberId);
        },
        [PM_ACTIONS.CLEAR_SELECTION]: function ({commit}: any) {
            commit(PROJECT_MEMBER_MUTATIONS.CLEAR_SELECTION);
        },
    },
    getters: {
        selectedProjectMembers: (state: any) => state.page.projectMembers.filter((member: any) => member.isSelected),
    }
};