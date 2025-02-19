// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive } from 'vue';
import { defineStore } from 'pinia';

import {
    ProjectMember,
    ProjectMemberCursor,
    ProjectMemberItemModel,
    ProjectMemberOrderBy,
    ProjectMembersApi,
    ProjectMembersPage,
    ProjectRole,
} from '@/types/projectMembers';
import { ProjectMembersHttpApi } from '@/api/projectMembers';
import { SortDirection } from '@/types/common';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { useConfigStore } from '@/store/modules/configStore';

export class ProjectMembersState {
    public cursor: ProjectMemberCursor = new ProjectMemberCursor();
    public page: ProjectMembersPage = new ProjectMembersPage();
    public selectedProjectMembersEmails: string[] = [];
    public lastProjectID = '';
}

export const useProjectMembersStore = defineStore('projectMembers', () => {
    const state = reactive<ProjectMembersState>(new ProjectMembersState());

    const api: ProjectMembersApi = new ProjectMembersHttpApi();

    const configStore = useConfigStore();
    const csrfToken = computed<string>(() => configStore.state.config.csrfToken);

    async function inviteMember(email: string, projectID: string): Promise<void> {
        await api.invite(projectID, email, csrfToken.value);
    }

    async function reinviteMembers(emails: string[], projectID: string): Promise<void> {
        await api.reinvite(projectID, emails, csrfToken.value);
    }

    async function getInviteLink(email: string, projectID: string): Promise<string> {
        return await api.getInviteLink(projectID, email);
    }

    async function getSingleMember(projectID: string, memberID: string): Promise<ProjectMember> {
        return await api.getSingleMember(projectID, memberID);
    }

    async function updateRole(projectID: string, memberID: string, role: ProjectRole): Promise<void> {
        const updatedMember = await api.updateRole(projectID, memberID, role, csrfToken.value);

        state.page.projectMembers?.forEach(pr => {
            if (pr.id === updatedMember.id) pr.role = updatedMember.role;
        });
    }

    async function deleteProjectMembers(projectID: string, customSelected?: string[]): Promise<void> {
        if (customSelected && customSelected.length) {
            await api.delete(projectID, customSelected, csrfToken.value);
            return;
        }

        await api.delete(projectID, state.selectedProjectMembersEmails, csrfToken.value);

        clearProjectMemberSelection();
    }

    async function getProjectMembers(page: number, projectID: string, limit = DEFAULT_PAGE_LIMIT): Promise<ProjectMembersPage> {
        state.cursor.page = page;
        state.cursor.limit = limit;
        state.lastProjectID = projectID;

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

    async function refresh(): Promise<void> {
        clearProjectMemberSelection();
        await getProjectMembers(state.cursor.page, state.lastProjectID, state.cursor.limit);
    }

    function clear() {
        state.cursor = new ProjectMemberCursor();
        state.page = new ProjectMembersPage();
        state.selectedProjectMembersEmails = [];
    }

    return {
        state,
        inviteMember,
        getSingleMember,
        updateRole,
        reinviteMembers,
        getInviteLink,
        deleteProjectMembers,
        getProjectMembers,
        setSearchQuery,
        setSortingBy,
        setSortingDirection,
        setPage,
        setPageNumber,
        toggleProjectMemberSelection,
        clearProjectMemberSelection,
        refresh,
        clear,
    };
});
