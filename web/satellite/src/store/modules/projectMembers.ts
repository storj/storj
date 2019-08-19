// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { PROJECT_MEMBER_MUTATIONS } from '@/store/mutationConstants';
import {
    ProjectMember,
    ProjectMemberCursor,
    ProjectMemberOrderBy,
    ProjectMembersApi,
    ProjectMembersPage
} from '@/types/projectMembers';
import { RequestResponse } from '@/types/response';
import { SortDirection } from '@/types/common';
import { StoreModule } from '@/store';

const {
    FETCH,
    TOGGLE_SELECTION,
    CLEAR_SELECTION,
    CLEAR,
    CHANGE_SORT_ORDER,
    CHANGE_SORT_ORDER_DIRECTION,
    SET_SEARCH_QUERY,
    SET_PAGE,
} = PROJECT_MEMBER_MUTATIONS;

class ProjectMembersState {
    public cursor: ProjectMemberCursor = new ProjectMemberCursor();
    public page: ProjectMembersPage = new ProjectMembersPage();
}

const PROJECT_MEMBERS_PAGE_LIMIT = 8;
const FIRST_PAGE = 1;

export function makeProjectMembersModule(api: ProjectMembersApi): StoreModule<ProjectMembersState> {
    return {
        state: new ProjectMembersState(),
        mutations: {
            fetchProjectMembers(state: any, page: ProjectMembersPage) {
                state.page = page;
            },
            setProjectMembersPage(state: any, page: number) {
                state.cursor.page = page;
            },
            setProjectMembersSearchQuery(state: any, search: string) {
                state.cursor.search = search;
            },
            changeProjectMembersSortOrder(state: any, order: ProjectMemberOrderBy) {
                state.cursor.order = order;
            },
            changeProjectMembersSortOrderDirection(state: any, direction: SortDirection) {
                state.cursor.orderDirection = direction;
            },
            clearProjectMembers(state: any) {
                state.cursor = {limit: PROJECT_MEMBERS_PAGE_LIMIT, search: '', page: FIRST_PAGE} as ProjectMemberCursor;
                state.page = {projectMembers: [] as ProjectMember[]} as ProjectMembersPage;
            },
            toggleSelection(state: any, projectMemberId: string) {
                state.page.projectMembers = state.page.projectMembers.map((projectMember: any) => {
                    if (projectMember.user.id === projectMemberId) {
                        projectMember.isSelected = !projectMember.isSelected;
                    }

                    return projectMember;
                });
            },
            clearSelection(state: any) {
                state.page.projectMembers = state.page.projectMembers.map((projectMember: any) => {
                    projectMember.isSelected = false;

                    return projectMember;
                });
            },
        },
        actions: {
            addProjectMembers: async function ({rootGetters}: any, emails: string[]): Promise<null> {
                const projectId = rootGetters.selectedProject.id;

                return await api.add(projectId, emails);
            },
            deleteProjectMembers: async function ({commit, rootGetters}: any, projectMemberEmails: string[]): Promise<null> {
                const projectId = rootGetters.selectedProject.id;

                return await api.delete(projectId, projectMemberEmails);
            },
            fetchProjectMembers: async function ({commit, rootGetters, state}: any, page: number): Promise<ProjectMembersPage> {
                const projectID = rootGetters.selectedProject.id;
                state.cursor.page = page;

                commit(SET_PAGE, page);

                const projectMembersPage: ProjectMembersPage = await api.get(projectID, state.cursor);

                commit(FETCH, projectMembersPage);

                return projectMembersPage;
            },
            setProjectMembersSearchQuery: function ({commit}, search: string) {
                commit(SET_SEARCH_QUERY, search);
            },
            setProjectMembersSortingBy: function ({commit}, order: ProjectMemberOrderBy) {
                commit(CHANGE_SORT_ORDER, order);
            },
            setProjectMembersSortingDirection: function ({commit}, direction: SortDirection) {
                commit(CHANGE_SORT_ORDER_DIRECTION, direction);
            },
            clearProjectMembers: function ({commit}) {
                commit(CLEAR);
            },
            toggleProjectMemberSelection: function ({commit}: any, projectMemberId: string) {
                commit(TOGGLE_SELECTION, projectMemberId);
            },
            clearProjectMemberSelection: function ({commit}: any) {
                commit(CLEAR_SELECTION);
            },
        },
        getters: {
            selectedProjectMembers: (state: any) => state.page.projectMembers.filter((member: any) => member.isSelected),
        }
    };
}
