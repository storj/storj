// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import { projectMembersModule } from '@/store/modules/projectMembers';
import {
	addProjectMembersRequest,
	deleteProjectMembersRequest,
	fetchProjectMembersRequest
} from '@/api/projectMembers';
import { PROJECT_MEMBER_MUTATIONS } from '@/store/mutationConstants';

describe('mutations', () => {
	it('success delete project members', () => {
		const state = {
			projectMembers: [{user: {email: '1'}}, {user: {email: '2'}}]
		};
		const membersToDelete = ['1', '2'];

		projectMembersModule.mutations.DELETE_MEMBERS(state, membersToDelete);

		expect(state.projectMembers.length).toBe(0);
	});

	it('error delete project members', () => {
		const state = {
			projectMembers: [{user: {email: '1'}}, {user: {email: '2'}}]
		};
		const membersToDelete = ['3', '4'];

		projectMembersModule.mutations.DELETE_MEMBERS(state, membersToDelete);

		expect(state.projectMembers.length).toBe(2);
	});

	it('toggle selection', () => {
		const state = {
			projectMembers: [{
				user: {id: '1'},
				isSelected: false
			}, {
				user: {id: '2'},
				isSelected: false
			}]
		};
		const memberId = '1';

		projectMembersModule.mutations.TOGGLE_SELECTION(state, memberId);

		expect(state.projectMembers[0].isSelected).toBeTruthy();
		expect(state.projectMembers[1].isSelected).toBeFalsy();
	});

	it('clear selection', () => {
		const state = {
			projectMembers: [{
				user: {id: '1'},
				isSelected: true
			}, {
				user: {id: '2'},
				isSelected: true
			}]
		};

		projectMembersModule.mutations.CLEAR_SELECTION(state);

		expect(state.projectMembers[0].isSelected).toBeFalsy();
		expect(state.projectMembers[1].isSelected).toBeFalsy();
	});

	it('fetch team members', () => {
		const state = {
			projectMembers: []
		};
		const teamMembers = [{
			user: {id: '1'}
		}];

		projectMembersModule.mutations.FETCH_MEMBERS(state, teamMembers);

		expect(state.projectMembers.length).toBe(1);
	});
});

describe('actions', () => {
	it('success add project members', async function () {
		const rootGetters = {
			selectedProject: {
				id: '1'
			}
		};
		addProjectMembersRequest = jest.fn().mockReturnValue({isSuccess: true});
		const emails = ['1', '2'];

		const dispatchResponse = await projectMembersModule.actions.addProjectMembers({rootGetters}, emails);

		expect(dispatchResponse.isSuccess).toBeTruthy();
	});

	it('error add project members', async function () {
		const rootGetters = {
			selectedProject: {
				id: '1'
			}
		};
		addProjectMembersRequest = jest.fn().mockReturnValue({isSuccess: false});
		const emails = ['1', '2'];

		const dispatchResponse = await projectMembersModule.actions.addProjectMembers({rootGetters}, emails);

		expect(dispatchResponse.isSuccess).toBeFalsy();
	});

	it('success delete project members', async () => {
		const rootGetters = {
				selectedProject: {
					id: '1'
				}
			}
		;
		deleteProjectMembersRequest = jest.fn().mockReturnValue({isSuccess: true});
		const commit = jest.fn();
		const emails = ['1', '2'];

		const dispatchResponse = await projectMembersModule.actions.deleteProjectMembers({commit, rootGetters}, emails);

		expect(dispatchResponse.isSuccess).toBeTruthy();
		expect(commit).toHaveBeenCalledWith(PROJECT_MEMBER_MUTATIONS.DELETE, emails);
	});

	it('error delete project members', async () => {
		const rootGetters = {
			selectedProject: {
				id: '1'
			}
		};
		deleteProjectMembersRequest = jest.fn().mockReturnValue({isSuccess: false});
		const commit = jest.fn();
		const emails = ['1', '2'];

		const dispatchResponse = await projectMembersModule.actions.deleteProjectMembers({commit, rootGetters}, emails);

		expect(dispatchResponse.isSuccess).toBeFalsy();
		expect(commit).toHaveBeenCalledTimes(0);
	});

	it('toggle selection', function () {
		const commit = jest.fn();
		const projectMemberId = '1';

		projectMembersModule.actions.toggleProjectMemberSelection({commit}, projectMemberId);

		expect(commit).toHaveBeenCalledWith(PROJECT_MEMBER_MUTATIONS.TOGGLE_SELECTION, projectMemberId);
	});

	it('clear selection', function () {
		const commit = jest.fn();

		projectMembersModule.actions.clearProjectMemberSelection({commit});

		expect(commit).toHaveBeenCalledTimes(1);
	});

	it('success fetch project members', async function () {
		const rootGetters = {
			selectedProject: {
				id: '1'
			}
		};
		const commit = jest.fn();
		const projectMemberMockModel: TeamMemberModel = {
			user: {
				id: '1',
				email: '1',
				firstName: '1',
				lastName: '1'
			},
			joinedAt: '1'
		};
		fetchProjectMembersRequest = jest.fn().mockReturnValue({isSuccess: true, data: [projectMemberMockModel]});

		const dispatchResponse = await projectMembersModule.actions.fetchProjectMembers({
			commit,
			rootGetters
		}, {limit: '1', offset: '0'});

		expect(dispatchResponse.isSuccess).toBeTruthy();
		expect(commit).toHaveBeenCalledWith(PROJECT_MEMBER_MUTATIONS.FETCH, [projectMemberMockModel]);
	});

	it('error fetch project members', async function () {
		const rootGetters = {
			selectedProject: {
				id: '1'
			}
		};
		const commit = jest.fn();
		fetchProjectMembersRequest = jest.fn().mockReturnValue({isSuccess: false});

		const dispatchResponse = await projectMembersModule.actions.fetchProjectMembers({
			commit,
			rootGetters
		}, {limit: '1', offset: '0'});

		expect(dispatchResponse.isSuccess).toBeFalsy();
		expect(commit).toHaveBeenCalledTimes(0);
	});
});

describe('getters', () => {
	it('project members', function () {
		const state = {
			projectMembers: [{user: {email: '1'}}]
		};
		const retrievedProjectMembers = projectMembersModule.getters.projectMembers(state);

		expect(retrievedProjectMembers.length).toBe(1);
	});

	it('selected project members', function () {
		const state = {
			projectMembers: [
				{isSelected: false},
				{isSelected: true},
				{isSelected: true},
				{isSelected: true},
				{isSelected: false},
			]
		};
		const retrievedSelectedProjectMembers = projectMembersModule.getters.selectedProjectMembers(state);
		expect(retrievedSelectedProjectMembers.length).toBe(3);
	});
});
