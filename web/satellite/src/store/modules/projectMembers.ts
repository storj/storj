// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import { PROJECT_MEMBER_MUTATIONS } from '../mutationConstants';
import { addProjectMember, deleteProjectMember, fetchProjectMembers } from '@/api/projectMembers';

export const projectMembersModule = {
	state: {
		projectMembers: [],
	},
	mutations: {
		[PROJECT_MEMBER_MUTATIONS.ADD](state: any, projectMember: TeamMemberModel) {
			state.projectMembers.push(projectMember);
		},
		[PROJECT_MEMBER_MUTATIONS.DELETE](state: any, projectMemberId: string) {

			const projectMember = state.projectMembers
				.find((teamMember: TeamMemberModel) => teamMember.user.id === projectMemberId);

			const index = state.projectMembers.indexOf(projectMember, 0);
			if (index === -1) {
				// TODO: Notify about error

				return;
			}

			state.projectMembers.splice(index, 1);
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
			state.projectMembers = teamMembers;
		},
	},
	actions: {
		addProjectMember: async function ({commit, rootState}: any, userId: string): Promise<boolean> {
			const projectId = rootState.projectsModule.state.selectedProject.id;
			const response = await addProjectMember(userId, projectId);

			if (!response || !response.data) {
				return false;
			}

			commit(PROJECT_MEMBER_MUTATIONS.ADD, response.data);

			return true;
		},
		deleteProjectMembers: async function ({commit, rootGetters}: any, projectMemberIds: string[]): Promise<boolean> {
			const projectId = rootGetters.selectedProject.id;
			let isDeletionSuccess = true;

			for await (const id of projectMemberIds) {
				const response = await deleteProjectMember(id, projectId);

				if (!response || !response.data) {
					isDeletionSuccess = false;
				}

				commit(PROJECT_MEMBER_MUTATIONS.DELETE, id);
			}

			return isDeletionSuccess;
		},
		toggleProjectMemberSelection: function ({commit}: any, projectMemberId: string) {
			commit(PROJECT_MEMBER_MUTATIONS.TOGGLE_SELECTION, projectMemberId);
		},
		clearProjectMemberSelection: function ({commit}: any) {
			commit(PROJECT_MEMBER_MUTATIONS.CLEAR_SELECTION);
		},
		fetchProjectMembers: async function ({commit, rootGetters}: any): Promise<boolean> {
			const projectId = rootGetters.selectedProject.id;
			const response = await fetchProjectMembers(projectId);

			if (!response || !response.data) {
				return false;
			}

			commit(PROJECT_MEMBER_MUTATIONS.FETCH, response.data.project.members);

			return true;
		},
	},
	getters: {
		projectMembers: (state: any) => state.projectMembers,
		selectedProjectMembers: (state: any) => state.projectMembers.filter((member: any) => member.isSelected)
	},
};
