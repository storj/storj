// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, reactive } from 'vue';

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
import { ProjectsApiGql } from '@/api/projects';
import { useUsersStore } from '@/store/modules/usersStore';

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

const PROJECT_PAGE_LIMIT = 7;

export const useProjectsStore = defineStore('projects', () => {
    const state = reactive<ProjectsState>(new ProjectsState());

    const api: ProjectsApi = new ProjectsApiGql();

    async function fetchProjects(): Promise<Project[]> {
        const projects = await api.get();

        setProjects(projects);

        return projects;
    }

    function setProjects(projects: Project[]): void {
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
    }

    async function fetchOwnedProjects(pageNumber: number): Promise<void> {
        state.cursor.page = pageNumber;
        state.cursor.limit = PROJECT_PAGE_LIMIT;

        state.page = await api.getOwnedProjects(state.cursor);
    }

    async function fetchDailyProjectData(payload: ProjectUsageDateRange): Promise<void> {
        const usage: ProjectsStorageBandwidthDaily = await api.getDailyUsage(state.selectedProject.id, payload.since, payload.before);

        state.allocatedBandwidthChartData = usage.allocatedBandwidth;
        state.settledBandwidthChartData = usage.settledBandwidth;
        state.storageChartData = usage.storage;
        state.chartDataSince = payload.since;
        state.chartDataBefore = payload.before;
    }

    async function createProject(createProjectFields: ProjectFields): Promise<string> {
        const createdProject = await api.create(createProjectFields);

        state.projects.push(createdProject);

        return createdProject.id;
    }

    async function createDefaultProject(): Promise<void> {
        const UNTITLED_PROJECT_NAME = 'My First Project';
        const UNTITLED_PROJECT_DESCRIPTION = '___';
        const { usersState } = useUsersStore();

        const project = new ProjectFields(
            UNTITLED_PROJECT_NAME,
            UNTITLED_PROJECT_DESCRIPTION,
            usersState.user.id,
        );

        const createdProjectId = await createProject(project);

        selectProject(createdProjectId);
    }

    function selectProject(projectID: string): void {
        const selected = state.projects.find((project: Project) => project.id === projectID);

        if (!selected) {
            return;
        }

        state.selectedProject = selected;
    }

    async function updateProjectName(fieldsToUpdate: ProjectFields): Promise<void> {
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

        state.selectedProject.name = fieldsToUpdate.name;
    }

    async function updateProjectDescription(fieldsToUpdate: ProjectFields): Promise<void> {
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

        state.selectedProject.description = fieldsToUpdate.description;
    }

    async function updateProjectStorageLimit(limitsToUpdate: ProjectLimits): Promise<void> {
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

        state.currentLimits.storageLimit = limitsToUpdate.storageLimit;
    }

    async function updateProjectBandwidthLimit(limitsToUpdate: ProjectLimits): Promise<void> {
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

        state.currentLimits.bandwidthLimit = limitsToUpdate.bandwidthLimit;
    }

    async function deleteProject(projectID: string): Promise<void> {
        await api.delete(projectID);

        state.projects = state.projects.filter(project => project.id !== projectID);

        if (state.selectedProject.id === projectID) {
            state.selectedProject = new Project();
        }
    }

    async function fetchProjectLimits(projectID: string): Promise<void> {
        state.currentLimits = await api.getLimits(projectID);
    }

    async function fetchTotalLimits(): Promise<void> {
        state.totalLimits = await api.getTotalLimits();
    }

    async function getProjectSalt(projectID: string): Promise<string> {
        return await api.getSalt(projectID);
    }

    function clearProjectState(): void {
        state.projects = [];
        state.selectedProject = defaultSelectedProject;
        state.currentLimits = new ProjectLimits();
        state.totalLimits = new ProjectLimits();
        state.storageChartData = [];
        state.allocatedBandwidthChartData = [];
        state.settledBandwidthChartData = [];
        state.chartDataSince = new Date();
        state.chartDataBefore = new Date();
    }

    const projects = computed(() => {
        return state.projects.map((project: Project) => {
            if (project.id === state.selectedProject.id) {
                project.isSelected = true;
            }

            return project;
        });
    });

    const projectsWithoutSelected = computed(() => {
        return state.projects.filter((project: Project) => {
            return project.id !== state.selectedProject.id;
        });
    });

    const projectsCount = computed(() => {
        let projectsCount = 0;

        const { usersState } = useUsersStore();

        state.projects.forEach((project: Project) => {
            if (project.ownerId === usersState.user.id) {
                projectsCount++;
            }
        });

        return projectsCount;
    });

    const selectedProject = computed((): Project => {
        return state.selectedProject;
    });

    return {
        projectsState: state,
        fetchProjects,
        fetchOwnedProjects,
        fetchDailyProjectData,
        createProject,
        createDefaultProject,
        selectProject,
        updateProjectName,
        updateProjectDescription,
        updateProjectStorageLimit,
        updateProjectBandwidthLimit,
        deleteProject,
        fetchProjectLimits,
        fetchTotalLimits,
        getProjectSalt,
        clearProjectState,
        projects,
        projectsWithoutSelected,
        projectsCount,
        selectedProject,
    };
});
