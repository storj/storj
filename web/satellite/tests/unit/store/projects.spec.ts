import { projectsModule } from '@/store/modules/projects';
import { createProjectRequest, deleteProjectRequest, fetchProjectsRequest, updateProjectRequest } from '@/api/projects';
import { PROJECTS_MUTATIONS } from '@/store/mutationConstants';

describe('mutations', () => {
	it('create project', () => {
		const state = {
			projects: [],
		};
		const project = {
			name: 'testName',
		};
		projectsModule.mutations.CREATE_PROJECT(state, project);

		expect(state.projects.length).toBe(1);
		const mutatedProject: Project = state.projects[0];
		expect(mutatedProject.name).toBe('testName');
	});
	it('fetch project', () => {
		const state = {
			projects: []
		};
		const projectsToPush = [{id: '1'}, {id: '2'}];
		projectsModule.mutations.FETCH_PROJECTS(state, projectsToPush);
	:

		expect(state.projects.length).toBe(2);
	});

	it('success select project', () => {
		const state = {
			projects: [{id: '1'}, {id: 'testId'}, {id: '2'},],
			selectedProject: {
				id: ''
			}
		};
		const projectId = 'testId';
		projectsModule.mutations.SELECT_PROJECT(state, projectId);

		expect(state.selectedProject.id).toBe('testId');
	});
	it('error select project', () => {
		const state = {
			projects: [{id: '1'}, {id: 'testId'}, {id: '2'},],
			selectedProject: {
				id: 'old'
			}
		};
		const projectId = '3';
		projectsModule.mutations.SELECT_PROJECT(state, projectId);

		expect(state.selectedProject.id).toBe('old');
	});
	it('error update project', () => {
		const state = {
			projects: [{id: '1'}, {id: 'testId'}, {id: '2'},],
			selectedProject: {
				id: 'old'
			}
		};
		const projectId = {id: '3'};
		projectsModule.mutations.UPDATE_PROJECT(state, projectId);

		expect(state.selectedProject.id).toBe('old');
	});
	it('error update project', () => {
		const state = {
			projects: [{id: '1'}, {id: 'testId'}, {id: '2'},],
			selectedProject: {
				id: 'old',
				description: 'oldD'
			}
		};
		const project = {id: '2', description: 'newD'};
		projectsModule.mutations.UPDATE_PROJECT(state, project);

		expect(state.selectedProject.id).toBe('2');
		expect(state.selectedProject.description).toBe('newD');
	});
	it('success update project', () => {
		const state = {
			projects: [{id: '1'}, {id: 'testId'}, {id: '2'},],
			selectedProject: {
				id: '2',
				description: 'oldD'
			}
		};
		const project = {id: '2', description: 'newD'};
		projectsModule.mutations.UPDATE_PROJECT(state, project);

		expect(state.selectedProject.id).toBe('2');
		expect(state.selectedProject.description).toBe('newD');
	});
	it('error delete project', () => {
		const state = {
			selectedProject: {
				id: '1',
			}
		};
		const projectId = '2';
		projectsModule.mutations.DELETE_PROJECT(state, projectId);

		expect(state.selectedProject.id).toBe('1');

	});
	it('success delete project', () => {
		const state = {
			selectedProject: {
				id: '2',
			}
		};
		const projectId = '2';
		projectsModule.mutations.DELETE_PROJECT(state, projectId);

		expect(state.selectedProject.id).toBe('');
	});
});

describe('actions', () => {
	it('success fetch project', async () => {
		fetchProjectsRequest = jest.fn().mockReturnValue({isSuccess: true, data: [{id: '1'}, {id: '2'}]});
		const commit = jest.fn();
		const dispatchResponse = await projectsModule.actions.fetchProjects({commit});

		expect(dispatchResponse.isSuccess).toBeTruthy();
		expect(commit).toHaveBeenCalledWith(PROJECTS_MUTATIONS.FETCH, [{id: '1'}, {id: '2'}]);
	});
	it('error fetch project', async () => {
		fetchProjectsRequest = jest.fn().mockReturnValue({isSuccess: false});
		const commit = jest.fn();
		const dispatchResponse = await projectsModule.actions.fetchProjects({commit});

		expect(dispatchResponse.isSuccess).toBeTruthy();
		expect(commit).toHaveBeenCalledTimes(0);
	});
	it('success create project', async () => {
		createProjectRequest = jest.fn().mockReturnValue({isSuccess: true, data: {id: '1'}});
		const commit = jest.fn();
		const project: Project = {
			name: '',
			id: '',
			description: '',
			isTermsAccepted: false,
			isSelected: false,
			createdAt: ''
		};

		const dispatchResponse = await projectsModule.actions.createProject({commit}, project);

		expect(dispatchResponse.isSuccess).toBeTruthy();
		expect(commit).toHaveBeenCalledWith(PROJECTS_MUTATIONS.CREATE, {id: '1'});
	});
	it('error create project', async () => {
		createProjectRequest = jest.fn().mockReturnValue({isSuccess: false});
		const commit = jest.fn();
		const project: Project = {
			name: '',
			id: '',
			description: '',
			isTermsAccepted: false,
			isSelected: false,
			createdAt: ''
		};

		const dispatchResponse = await projectsModule.actions.createProject({commit}, project);

		expect(dispatchResponse.isSuccess).toBeFalsy();
		expect(commit).toHaveBeenCalledTimes(0);
	});
	it('success select project', () => {
		const commit = jest.fn();
		projectsModule.actions.selectProject({commit}, 'id');

		expect(commit).toHaveBeenCalledWith(PROJECTS_MUTATIONS.SELECT, 'id');
	});
	it('success update project description', async () => {
		updateProjectRequest = jest.fn().mockReturnValue({isSuccess: true});
		const commit = jest.fn();

		const project: Project = {
			name: '',
			id: 'id',
			description: 'desc',
			isTermsAccepted: false,
			isSelected: false,
			createdAt: ''
		};

		const dispatchResponse = await projectsModule.actions.updateProjectDescription({commit}, project);

		expect(dispatchResponse.isSuccess).toBeTruthy();
		expect(commit).toBeCalledWith(PROJECTS_MUTATIONS.UPDATE, project);
	});

	it('error update project description', async () => {
		updateProjectRequest = jest.fn().mockReturnValue({isSuccess: false});
		const commit = jest.fn();
		const project: Project = {
			name: '',
			id: '',
			description: '',
			isTermsAccepted: false,
			isSelected: false,
			createdAt: ''
		};

		const dispatchResponse = await projectsModule.actions.updateProjectDescription({commit}, project);

		expect(dispatchResponse.isSuccess).toBeFalsy();
		expect(commit).toHaveBeenCalledTimes(0);
	});
	it('success delete project', async () => {
		deleteProjectRequest = jest.fn().mockReturnValue({isSuccess: true});
		const commit = jest.fn();
		const project = 'id';

		const dispatchResponse = await projectsModule.actions.deleteProject({commit}, project);

		expect(dispatchResponse.isSuccess).toBeTruthy();
		expect(commit).toHaveBeenCalledWith(PROJECTS_MUTATIONS.DELETE, project);
	});
	it('error delete project', async () => {
		deleteProjectRequest = jest.fn().mockReturnValue({isSuccess: false});
		const commit = jest.fn();

		const dispatchResponse = await projectsModule.actions.deleteProject({commit}, 'id');

		expect(dispatchResponse.isSuccess).toBeFalsy();
		expect(commit).toHaveBeenCalledTimes(0);
	});
});

describe('getters', () => {

	it('getter projects', () => {
		const state = {
			projects: [{
				name: '1',
				id: '1',
				companyName: '1',
				description: '1',
				isTermsAccepted: true,
				createdAt: '1',
			}],
			selectedProject: {
				id: '1'
			}
		};

		const projectsGetterArray = projectsModule.getters.projects(state);

		expect(projectsGetterArray.length).toBe(1);

		const firstProject = projectsGetterArray[0];

		expect(firstProject.name).toBe('1');
		expect(firstProject.id).toBe('1');
		expect(firstProject.description).toBe('1');
		expect(firstProject.isTermsAccepted).toBe(true);
		expect(firstProject.createdAt).toBe('1');
	});
	it('getter projects', () => {
		const state = {
			projects: [{
				name: '1',
				id: '1',
				companyName: '1',
				description: '1',
				isTermsAccepted: true,
				createdAt: '1',
			}],
			selectedProject: {
				id: '2'
			}
		};

		const projectsGetterArray = projectsModule.getters.projects(state);

		expect(projectsGetterArray.length).toBe(1);

		const firstProject = projectsGetterArray[0];

		expect(firstProject.name).toBe('1');
		expect(firstProject.id).toBe('1');
		expect(firstProject.description).toBe('1');
		expect(firstProject.isTermsAccepted).toBe(true);
		expect(firstProject.createdAt).toBe('1');
	});
	it('getters selected project', () => {
		const state = {
			selectedProject: {
				name: '1',
				id: '1',
				description: '1',
				isTermsAccepted: true,
				createdAt: '1',
			}
		};
		const selectedProjectGetterObject = projectsModule.getters.selectedProject(state);

		expect(selectedProjectGetterObject.name).toBe('1');
		expect(selectedProjectGetterObject.id).toBe('1');
		expect(selectedProjectGetterObject.description).toBe('1');
		expect(selectedProjectGetterObject.isTermsAccepted).toBe(true);
		expect(selectedProjectGetterObject.createdAt).toBe('1');
	});
});
