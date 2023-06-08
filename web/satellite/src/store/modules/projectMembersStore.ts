// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import {
    ProjectMember,
    ProjectMemberCursor,
    ProjectMemberItemModel,
    ProjectMemberOrderBy,
    ProjectMembersApi,
    ProjectMembersPage,
} from '@/types/projectMembers';
import { ProjectMembersApiGql } from '@/api/projectMembers';
import { SortDirection } from '@/types/common';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

export class ProjectMembersState {
    public cursor: ProjectMemberCursor = new ProjectMemberCursor();
    public page: ProjectMembersPage = new ProjectMembersPage();
    public selectedProjectMembersEmails: string[] = [];
}

export const useProjectMembersStore = defineStore('projectMembers', () => {
    const state = reactive<ProjectMembersState>(new ProjectMembersState());

    const api: ProjectMembersApi = new ProjectMembersApiGql();

    async function inviteMembers(emails: string[], projectID: string): Promise<void> {
        await api.invite(projectID, emails);
    }

    async function deleteProjectMembers(projectID: string): Promise<void> {
        await api.delete(projectID, state.selectedProjectMembersEmails);

        clearProjectMemberSelection();
    }

    async function getProjectMembers(page: number, projectID: string, limit = DEFAULT_PAGE_LIMIT): Promise<ProjectMembersPage> {
        state.cursor.page = page;
        state.cursor.limit = limit;

        const projectMembersPage: ProjectMembersPage = await api.get(projectID, state.cursor);

        state.page = projectMembersPage;
        state.page.getAllItems().forEach(item => {
            item.setSelected(state.selectedProjectMembersEmails.includes(item.getEmail()));
        });

        return projectMembersPage;
    }

    function setPage(page: ProjectMembersPage) {
        state.page = page;
    }

    function setPageNumber(page: number) {
        state.cursor.page = page;
    }

    function setSearchQuery(search: string) {
        state.cursor.search = search;
    }

    function getSearchQuery() {
        return state.cursor.search;
    }

    function setSortingBy(order: ProjectMemberOrderBy) {
        state.cursor.order = order;
    }

    function setSortingDirection(direction: SortDirection) {
        state.cursor.orderDirection = direction;
    }

    function toggleProjectMemberSelection(projectMember: ProjectMemberItemModel) {
        const email = projectMember.getEmail();

        if (!state.selectedProjectMembersEmails.includes(email)) {
            projectMember.setSelected(true);
            state.selectedProjectMembersEmails.push(email);

            return;
        }

        projectMember.setSelected(false);
        state.selectedProjectMembersEmails = state.selectedProjectMembersEmails.filter(projectMemberEmail => {
            return projectMemberEmail !== email;
        });
    }

    function clearProjectMemberSelection() {
        state.selectedProjectMembersEmails = [];
        state.page.getAllItems().forEach(member => member.setSelected(false));
    }

    function clear() {
        state.cursor = new ProjectMemberCursor();
        state.page = new ProjectMembersPage();
        state.selectedProjectMembersEmails = [];
    }

    return {
        state,
        inviteMembers,
        deleteProjectMembers,
        getProjectMembers,
        setSearchQuery,
        getSearchQuery,
        setSortingBy,
        setSortingDirection,
        setPage,
        setPageNumber,
        toggleProjectMemberSelection,
        clearProjectMemberSelection,
        clear,
    };
});
