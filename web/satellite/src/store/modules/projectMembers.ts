// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import { PROJECT_MEMBER_MUTATIONS } from '../mutationConstants';
import { addProjectMembers, deleteProjectMembers, fetchProjectMembers } from '@/api/projectMembers';

export const projectMembersModule = {
	state: {
		projectMembers: [],
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
            state.projectMembers = teamMembers;
        },
        [PROJECT_MEMBER_MUTATIONS.CLEAR](state: any) {
            state.projectMembers = [];
        },
	},
	actions: {
		addProjectMembers: async function ({rootGetters}: any, emails: string[]): Promise<RequestResponse<null>> {
			const projectId = rootGetters.selectedProject.id;
			
			const response = await addProjectMembers(projectId, emails);

			return response;
		},
		deleteProjectMembers: async function ({commit, rootGetters}: any, projectMemberEmails: string[]): Promise<RequestResponse<null>> {
			const projectId = rootGetters.selectedProject.id;

			const response = await deleteProjectMembers(projectId, projectMemberEmails);

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
		fetchProjectMembers: async function ({commit, rootGetters}: any, limitoffset: any): Promise<RequestResponse<TeamMemberModel[]>> {
			const projectId = rootGetters.selectedProject.id;
			const response = await fetchProjectMembers(projectId, limitoffset.limit, limitoffset.offset);

			if (response.isSuccess) {
				commit(PROJECT_MEMBER_MUTATIONS.FETCH, response.data);
			}

			return response;
		},
		clearProjectMembers: function ({commit}: any) {
            commit(PROJECT_MEMBER_MUTATIONS.CLEAR);
		}
	},
	getters: {
		projectMembers: (state: any) => state.projectMembers,
		selectedProjectMembers: (state: any) => state.projectMembers.filter((member: any) => member.isSelected)
	},
};
