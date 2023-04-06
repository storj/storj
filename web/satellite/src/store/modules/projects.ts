// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    DataStamp,
    Project,
    ProjectFields,
    ProjectLimits,
    ProjectsApi,
    ProjectsCursor,
    ProjectsPage,
    ProjectsStorageBandwidthDaily,
    ProjectUsageDateRange,
} from '@/types/projects';
import { StoreModule } from '@/types/store';

export const PROJECTS_ACTIONS = {
    FETCH: 'fetchProjects',
    FETCH_OWNED: 'fetchOwnedProjects',
    FETCH_DAILY_DATA: 'fetchDailyData',
    CREATE: 'createProject',
    CREATE_DEFAULT_PROJECT: 'createDefaultProject',
    SELECT: 'selectProject',
    UPDATE_NAME: 'updateProjectName',
    UPDATE_DESCRIPTION: 'updateProjectDescription',
    UPDATE_STORAGE_LIMIT: 'updateProjectStorageLimit',
    UPDATE_BANDWIDTH_LIMIT: 'updateProjectBandwidthLimit',
    DELETE: 'deleteProject',
    CLEAR: 'clearProjects',
    GET_LIMITS: 'getProjectLimits',
    GET_TOTAL_LIMITS: 'getTotalLimits',
    GET_SALT: 'getSalt',
};

export const PROJECTS_MUTATIONS = {
    ADD: 'CREATE_PROJECT',
    REMOVE: 'DELETE_PROJECT',
    UPDATE_PROJECT_NAME: 'UPDATE_PROJECT_NAME',
    UPDATE_PROJECT_DESCRIPTION: 'UPDATE_PROJECT_DESCRIPTION',
    UPDATE_PROJECT_STORAGE_LIMIT: 'UPDATE_STORAGE_LIMIT',
    UPDATE_PROJECT_BANDWIDTH_LIMIT: 'UPDATE_BANDWIDTH_LIMIT',
    SET_PROJECTS: 'SET_PROJECTS',
    SELECT_PROJECT: 'SELECT_PROJECT',
    CLEAR_PROJECTS: 'CLEAR_PROJECTS',
    SET_LIMITS: 'SET_PROJECT_LIMITS',
    SET_TOTAL_LIMITS: 'SET_TOTAL_LIMITS',
    SET_PAGE_NUMBER: 'SET_PAGE_NUMBER',
    SET_PAGE: 'SET_PAGE',
    SET_DAILY_DATA: 'SET_DAILY_DATA',
    SET_CHARTS_DATE_RANGE: 'SET_CHARTS_DATE_RANGE',
};

const defaultSelectedProject = new Project('', '', '', '', '', true, 0);

export class ProjectsState {
    public projects: Project[] = [];
    public selectedProject: Project = defaultSelectedProject;
    public currentLimits: ProjectLimits = new ProjectLimits();
    public totalLimits: ProjectLimits = new ProjectLimits();
    public cursor: ProjectsCursor = new ProjectsCursor();
    public page: ProjectsPage = new ProjectsPage();
    public allocatedBandwidthChartData: DataStamp[] = [];
    public settledBandwidthChartData: DataStamp[] = [];
    public storageChartData: DataStamp[] = [];
    public chartDataSince: Date = new Date();
    public chartDataBefore: Date = new Date();
}

interface ProjectsContext {
    state: ProjectsState
    commit: (string, ...unknown) => void
    rootGetters: {
        user: {
            id: string
        }
    }
    dispatch: (string, ...unknown) => Promise<any> // eslint-disable-line @typescript-eslint/no-explicit-any
}

const {
    FETCH,
    FETCH_DAILY_DATA,
    CREATE,
    CREATE_DEFAULT_PROJECT,
    SELECT,
    UPDATE_NAME,
    UPDATE_DESCRIPTION,
    UPDATE_STORAGE_LIMIT,
    UPDATE_BANDWIDTH_LIMIT,
    DELETE,
    CLEAR,
    GET_LIMITS,
    GET_TOTAL_LIMITS,
    FETCH_OWNED,
    GET_SALT,
} = PROJECTS_ACTIONS;

const {
    ADD,
    REMOVE,
    UPDATE_PROJECT_NAME,
    UPDATE_PROJECT_DESCRIPTION,
    UPDATE_PROJECT_STORAGE_LIMIT,
    UPDATE_PROJECT_BANDWIDTH_LIMIT,
    SET_PROJECTS,
    SELECT_PROJECT,
    CLEAR_PROJECTS,
    SET_LIMITS,
    SET_TOTAL_LIMITS,
    SET_PAGE_NUMBER,
    SET_PAGE,
    SET_DAILY_DATA,
    SET_CHARTS_DATE_RANGE,
} = PROJECTS_MUTATIONS;
const projectsPageLimit = 7;

export function makeProjectsModule(api: ProjectsApi): StoreModule<ProjectsState, ProjectsContext> {
    return {
        state: new ProjectsState(),
        mutations: {
            [ADD](state: ProjectsState, createdProject: Project): void {
                state.projects.push(createdProject);
            },
            [SET_PROJECTS](state: ProjectsState, projects: Project[]): void {
                state.projects = projects;

                if (!state.selectedProject.id) {
                    return;
                }

                const projectsCount = state.projects.length;

                for (let i = 0; i < projectsCount; i++) {
                    const project = state.projects[i];

                    if (project.id !== state.selectedProject.id) {
                        continue;
                    }

                    state.selectedProject = project;

                    return;
                }

                state.selectedProject = defaultSelectedProject;
            },
            [SELECT_PROJECT](state: ProjectsState, projectID: string): void {
                const selected = state.projects.find((project: Project) => project.id === projectID);

                if (!selected) {
                    return;
                }

                state.selectedProject = selected;
            },
            [UPDATE_PROJECT_NAME](state: ProjectsState, fieldsToUpdate: ProjectFields): void {
                state.selectedProject.name = fieldsToUpdate.name;
            },
            [UPDATE_PROJECT_DESCRIPTION](state: ProjectsState, fieldsToUpdate: ProjectFields): void {
                state.selectedProject.description = fieldsToUpdate.description;
            },
            [UPDATE_PROJECT_STORAGE_LIMIT](state: ProjectsState, limitsToUpdate: ProjectLimits): void {
                state.currentLimits.storageLimit = limitsToUpdate.storageLimit;
            },
            [UPDATE_PROJECT_BANDWIDTH_LIMIT](state: ProjectsState, limitsToUpdate: ProjectLimits): void {
                state.currentLimits.bandwidthLimit = limitsToUpdate.bandwidthLimit;
            },
            [REMOVE](state: ProjectsState, projectID: string): void {
                state.projects = state.projects.filter(project => project.id !== projectID);

                if (state.selectedProject.id === projectID) {
                    state.selectedProject = new Project();
                }
            },
            [SET_LIMITS](state: ProjectsState, limits: ProjectLimits): void {
                state.currentLimits = limits;
            },
            [SET_TOTAL_LIMITS](state: ProjectsState, limits: ProjectLimits): void {
                state.totalLimits = limits;
            },
            [CLEAR_PROJECTS](state: ProjectsState): void {
                state.projects = [];
                state.selectedProject = defaultSelectedProject;
                state.currentLimits = new ProjectLimits();
                state.totalLimits = new ProjectLimits();
                state.storageChartData = [];
                state.allocatedBandwidthChartData = [];
                state.settledBandwidthChartData = [];
                state.chartDataSince = new Date();
                state.chartDataBefore = new Date();
                state.cursor = new ProjectsCursor();
                state.page = new ProjectsPage();
            },
            [SET_PAGE_NUMBER](state: ProjectsState, pageNumber: number) {
                state.cursor.page = pageNumber;
                state.cursor.limit = projectsPageLimit;
            },
            [SET_PAGE](state: ProjectsState, page: ProjectsPage) {
                state.page = page;
            },
            [SET_DAILY_DATA](state: ProjectsState, payload: ProjectsStorageBandwidthDaily) {
                state.allocatedBandwidthChartData = payload.allocatedBandwidth;
                state.settledBandwidthChartData = payload.settledBandwidth;
                state.storageChartData = payload.storage;
            },
            [SET_CHARTS_DATE_RANGE](state: ProjectsState, payload: ProjectUsageDateRange) {
                state.chartDataSince = payload.since;
                state.chartDataBefore = payload.before;
            },
        },
        actions: {
            [FETCH]: async function ({ commit }: ProjectsContext): Promise<Project[]> {
                const projects = await api.get();

                commit(SET_PROJECTS, projects);

                return projects;
            },
            [FETCH_OWNED]: async function ({ commit, state }: ProjectsContext, pageNumber: number): Promise<ProjectsPage> {
                commit(SET_PAGE_NUMBER, pageNumber);

                const projectsPage: ProjectsPage = await api.getOwnedProjects(state.cursor);
                commit(SET_PAGE, projectsPage);

                return projectsPage;
            },
            [FETCH_DAILY_DATA]: async function ({ commit, state }: ProjectsContext, payload: ProjectUsageDateRange): Promise<void> {
                const usage: ProjectsStorageBandwidthDaily = await api.getDailyUsage(state.selectedProject.id, payload.since, payload.before);

                commit(SET_CHARTS_DATE_RANGE, payload);
                commit(SET_DAILY_DATA, usage);
            },
            [CREATE]: async function ({ commit }: ProjectsContext, createProjectFields: ProjectFields): Promise<Project> {
                const project = await api.create(createProjectFields);

                commit(ADD, project);

                return project;
            },
            [CREATE_DEFAULT_PROJECT]: async function ({ rootGetters, dispatch }: ProjectsContext, userID: string): Promise<void> {
                const UNTITLED_PROJECT_NAME = 'My First Project';
                const UNTITLED_PROJECT_DESCRIPTION = '___';
                const project = new ProjectFields(
                    UNTITLED_PROJECT_NAME,
                    UNTITLED_PROJECT_DESCRIPTION,
                    userID,
                );
                const createdProject = await dispatch(CREATE, project);

                await dispatch(SELECT, createdProject.id);
            },
            [SELECT]: function ({ commit }: ProjectsContext, projectID: string): void {
                commit(SELECT_PROJECT, projectID);
            },
            [UPDATE_NAME]: async function ({ commit, state }: ProjectsContext, fieldsToUpdate: ProjectFields): Promise<void> {
                const project = new ProjectFields(
                    fieldsToUpdate.name,
                    state.selectedProject.description,
                    state.selectedProject.id,
                );
                const limit = new ProjectLimits(
                    state.currentLimits.bandwidthLimit,
                    state.currentLimits.bandwidthUsed,
                    state.currentLimits.storageLimit,
                    state.currentLimits.storageUsed,
                );
                await api.update(state.selectedProject.id, project, limit);

                commit(UPDATE_PROJECT_NAME, fieldsToUpdate);
            },
            [UPDATE_DESCRIPTION]: async function ({ commit, state }: ProjectsContext, fieldsToUpdate: ProjectFields): Promise<void> {
                const project = new ProjectFields(
                    state.selectedProject.name,
                    fieldsToUpdate.description,
                    state.selectedProject.id,
                );
                const limit = new ProjectLimits(
                    state.currentLimits.bandwidthLimit,
                    state.currentLimits.bandwidthUsed,
                    state.currentLimits.storageLimit,
                    state.currentLimits.storageUsed,
                );
                await api.update(state.selectedProject.id, project, limit);

                commit(UPDATE_PROJECT_DESCRIPTION, fieldsToUpdate);
            },
            [UPDATE_STORAGE_LIMIT]: async function ({ commit, state }: ProjectsContext, limitsToUpdate: ProjectLimits): Promise<void> {
                const project = new ProjectFields(
                    state.selectedProject.name,
                    state.selectedProject.description,
                    state.selectedProject.id,
                );
                const limit = new ProjectLimits(
                    state.currentLimits.bandwidthLimit,
                    state.currentLimits.bandwidthUsed,
                    limitsToUpdate.storageLimit,
                    state.currentLimits.storageUsed,
                );
                await api.update(state.selectedProject.id, project, limit);

                commit(UPDATE_PROJECT_STORAGE_LIMIT, limitsToUpdate);
            },
            [UPDATE_BANDWIDTH_LIMIT]: async function ({ commit, state }: ProjectsContext, limitsToUpdate: ProjectLimits): Promise<void> {
                const project = new ProjectFields(
                    state.selectedProject.name,
                    state.selectedProject.description,
                    state.selectedProject.id,
                );
                const limit = new ProjectLimits(
                    limitsToUpdate.bandwidthLimit,
                    state.currentLimits.bandwidthUsed,
                    state.currentLimits.storageLimit,
                    state.currentLimits.storageUsed,
                );
                await api.update(state.selectedProject.id, project, limit);

                commit(UPDATE_PROJECT_BANDWIDTH_LIMIT, limitsToUpdate);
            },
            [DELETE]: async function ({ commit }: ProjectsContext, projectID: string): Promise<void> {
                await api.delete(projectID);

                commit(REMOVE, projectID);
            },
            [GET_LIMITS]: async function ({ commit }: ProjectsContext, projectID: string): Promise<ProjectLimits> {
                const limits = await api.getLimits(projectID);

                commit(SET_LIMITS, limits);

                return limits;
            },
            [GET_TOTAL_LIMITS]: async function ({ commit }: ProjectsContext): Promise<ProjectLimits> {
                const limits = await api.getTotalLimits();

                commit(SET_TOTAL_LIMITS, limits);

                return limits;
            },
            [GET_SALT]: async function (_, projectID: string): Promise<string> {
                return await api.getSalt(projectID);
            },
            [CLEAR]: function({ commit }: ProjectsContext): void {
                commit(CLEAR_PROJECTS);
            },
        },
        getters: {
            projects: (state: ProjectsState): Project[] => {
                return state.projects.map((project: Project) => {
                    if (project.id === state.selectedProject.id) {
                        project.isSelected = true;
                    }

                    return project;
                });
            },
            projectsWithoutSelected: (state: ProjectsState): Project[] => {
                return state.projects.filter((project: Project) => {
                    return project.id !== state.selectedProject.id;
                });
            },
            selectedProject: (state: ProjectsState): Project => state.selectedProject,
            projectsCount: (state: ProjectsState) => (userID: string): number => {
                let projectsCount = 0;

                state.projects.forEach((project: Project) => {
                    if (project.ownerId === userID) {
                        projectsCount++;
                    }
                });

                return projectsCount;
            },
        },
    };
}
