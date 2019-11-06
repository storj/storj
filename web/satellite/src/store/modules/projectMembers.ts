// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import { SortDirection } from '@/types/common';
import {
    ProjectMember,
    ProjectMemberCursor,
    ProjectMemberOrderBy,
    ProjectMembersApi,
    ProjectMembersPage,
} from '@/types/projectMembers';

export const PROJECT_MEMBER_MUTATIONS = {
    FETCH: 'fetchProjectMembers',
    TOGGLE_SELECTION: 'toggleSelection',
    CLEAR_SELECTION: 'clearSelection',
    CLEAR: 'clearProjectMembers',
    CHANGE_SORT_ORDER: 'changeProjectMembersSortOrder',
    CHANGE_SORT_ORDER_DIRECTION: 'changeProjectMembersSortOrderDirection',
    SET_SEARCH_QUERY: 'setProjectMembersSearchQuery',
    SET_PAGE: 'setProjectMembersPage',
};

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

export class ProjectMembersState {
    public cursor: ProjectMemberCursor = new ProjectMemberCursor();
    public page: ProjectMembersPage = new ProjectMembersPage();
    public selectedProjectMembersEmails: string[] = [];
}

export function makeProjectMembersModule(api: ProjectMembersApi): StoreModule<ProjectMembersState> {
    return {
        state: new ProjectMembersState(),
        mutations: {
            [FETCH](state: ProjectMembersState, page: ProjectMembersPage) {
                state.page = page;
                state.page.projectMembers = state.page.projectMembers.map(member => {
                    if (state.selectedProjectMembersEmails.includes(member.user.email)) {
                        member.isSelected = true;
                    }

                    return member;
                });
            },
            [SET_PAGE](state: ProjectMembersState, page: number) {
                state.cursor.page = page;
            },
            [SET_SEARCH_QUERY](state: ProjectMembersState, search: string) {
                state.cursor.search = search;
            },
            [CHANGE_SORT_ORDER](state: ProjectMembersState, order: ProjectMemberOrderBy) {
                state.cursor.order = order;
            },
            [CHANGE_SORT_ORDER_DIRECTION](state: ProjectMembersState, direction: SortDirection) {
                state.cursor.orderDirection = direction;
            },
            [CLEAR](state: ProjectMembersState) {
                state.cursor = new ProjectMemberCursor();
                state.page = new ProjectMembersPage();
                state.selectedProjectMembersEmails = [];
            },
            [TOGGLE_SELECTION](state: ProjectMembersState, projectMember: ProjectMember) {
                if (!state.selectedProjectMembersEmails.includes(projectMember.user.email)) {
                    projectMember.isSelected = true;
                    state.selectedProjectMembersEmails.push(projectMember.user.email);

                    return;
                }

                projectMember.isSelected = false;
                state.selectedProjectMembersEmails = state.selectedProjectMembersEmails.filter(projectMemberEmail => {
                    return projectMemberEmail !== projectMember.user.email;
                });
            },
            [CLEAR_SELECTION](state: ProjectMembersState) {
                state.selectedProjectMembersEmails = [];
                state.page.projectMembers = state.page.projectMembers.map((projectMember: ProjectMember) => {
                    projectMember.isSelected = false;

                    return projectMember;
                });
            },
        },
        actions: {
            addProjectMembers: async function ({rootGetters}: any, emails: string[]): Promise<void> {
                const projectId = rootGetters.selectedProject.id;

                await api.add(projectId, emails);
            },
            deleteProjectMembers: async function ({rootGetters, state, commit}: any): Promise<void> {
                const projectId = rootGetters.selectedProject.id;

                await api.delete(projectId, state.selectedProjectMembersEmails);

                commit(CLEAR_SELECTION);
            },
            fetchProjectMembers: async function ({commit, rootGetters, state}: any, page: number): Promise<ProjectMembersPage> {
                const projectID = rootGetters.selectedProject.id;

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
                commit(CLEAR_SELECTION);
            },
            toggleProjectMemberSelection: function ({commit}: any, projectMember: ProjectMember) {
                commit(TOGGLE_SELECTION, projectMember);
            },
            clearProjectMemberSelection: function ({commit}: any) {
                commit(CLEAR_SELECTION);
            },
        },
        getters: {
            selectedProjectMembers: (state: ProjectMembersState) =>
                state.page.projectMembers.filter((member: ProjectMember) =>
                    state.selectedProjectMembersEmails.includes(member.user.email)),
        },
    };
}
