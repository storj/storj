// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { PROJECT_MEMBER_MUTATIONS } from '../mutationConstants';
import {
    addProjectMembersRequest,
    deleteProjectMembersRequest,
    fetchProjectMembersRequest
} from '@/api/projectMembers';
import { ProjectMemberSortByEnum } from '@/utils/constants/ProjectMemberSortEnum';

export const projectMembersModule = {
    state: {
        projectMembers: [],
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
            state.projectMembers = state.projectMembers.concat(teamMembers);
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

            const response = await addProjectMembersRequest(projectId, emails);

            return response;
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
        selectedProjectMembers: (state: any) => state.projectMembers.filter((member: any) => member.isSelected),
    },
};
