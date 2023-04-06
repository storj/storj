// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import {
    ProjectMember,
    ProjectMemberCursor,
    ProjectMemberOrderBy,
    ProjectMembersApi,
    ProjectMembersPage,
} from '@/types/projectMembers';
import { ProjectMembersApiGql } from '@/api/projectMembers';
import { SortDirection } from '@/types/common';

export class ProjectMembersState {
    public cursor: ProjectMemberCursor = new ProjectMemberCursor();
    public page: ProjectMembersPage = new ProjectMembersPage();
    public selectedProjectMembersEmails: string[] = [];
}

export const useProjectMembersStore = defineStore('projectMembers', () => {
    const state = reactive<ProjectMembersState>(new ProjectMembersState());

    const api: ProjectMembersApi = new ProjectMembersApiGql();

    async function addProjectMembers(emails: string[], projectID: string): Promise<void> {
        await api.add(projectID, emails);
    }

    async function deleteProjectMembers(projectID: string): Promise<void> {
        await api.delete(projectID, state.selectedProjectMembersEmails);

        clearProjectMemberSelection();
    }

    async function getProjectMembers(page: number, projectID: string): Promise<ProjectMembersPage> {
        state.cursor.page = page;

        const projectMembersPage: ProjectMembersPage = await api.get(projectID, state.cursor);

        state.page = projectMembersPage;
        state.page.projectMembers = state.page.projectMembers.map(member => {
            if (state.selectedProjectMembersEmails.includes(member.user.email)) {
                member.isSelected = true;
            }

            return member;
        });

        return projectMembersPage;
    }

    function setProjectMembersSearchQuery(search: string) {
        state.cursor.search = search;
    }

    function setProjectMembersSortingBy(order: ProjectMemberOrderBy) {
        state.cursor.order = order;
    }

    function setProjectMembersSortingDirection(direction: SortDirection) {
        state.cursor.orderDirection = direction;
    }

    function toggleProjectMemberSelection(projectMember: ProjectMember) {
        if (!state.selectedProjectMembersEmails.includes(projectMember.user.email)) {
            projectMember.isSelected = true;
            state.selectedProjectMembersEmails.push(projectMember.user.email);

            return;
        }

        projectMember.isSelected = false;
        state.selectedProjectMembersEmails = state.selectedProjectMembersEmails.filter(projectMemberEmail => {
            return projectMemberEmail !== projectMember.user.email;
        });
    }

    function clearProjectMemberSelection() {
        state.selectedProjectMembersEmails = [];
        state.page.projectMembers = state.page.projectMembers.map((projectMember: ProjectMember) => {
            projectMember.isSelected = false;

            return projectMember;
        });
    }

    function clear() {
        state.cursor = new ProjectMemberCursor();
        state.page = new ProjectMembersPage();
        state.selectedProjectMembersEmails = [];
    }

    return {
        pmState: state,
    };
});
