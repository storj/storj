// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, reactive, readonly } from 'vue';

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
    ProjectInvitation,
    ProjectInvitationResponse,
} from '@/types/projects';
import { ProjectsHttpApi } from '@/api/projects';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

const DEFAULT_PROJECT = new Project('', '', '', '', '', true, 0);
const DEFAULT_INVITATION = new ProjectInvitation('', '', '', '', new Date());
export const DEFAULT_PROJECT_LIMITS = readonly(new ProjectLimits());

export class ProjectsState {
    public projects: Project[] = [];
    public selectedProject: Project = DEFAULT_PROJECT;
    public currentLimits: Readonly<ProjectLimits> = DEFAULT_PROJECT_LIMITS;
    public totalLimits: Readonly<ProjectLimits> = DEFAULT_PROJECT_LIMITS;
    public cursor: ProjectsCursor = new ProjectsCursor();
    public page: ProjectsPage = new ProjectsPage();
    public allocatedBandwidthChartData: DataStamp[] = [];
    public storageChartData: DataStamp[] = [];
    public chartDataSince: Date = new Date();
    public chartDataBefore: Date = new Date();
    public invitations: ProjectInvitation[] = [];
    public selectedInvitation: ProjectInvitation = DEFAULT_INVITATION;
}

export const useProjectsStore = defineStore('projects', () => {
    const state = reactive<ProjectsState>(new ProjectsState());

    const api: ProjectsApi = new ProjectsHttpApi();

    async function getProjects(): Promise<Project[]> {
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

        state.selectedProject = DEFAULT_PROJECT;
    }

    async function getOwnedProjects(pageNumber: number, limit = DEFAULT_PAGE_LIMIT): Promise<void> {
        state.cursor.page = pageNumber;
        state.cursor.limit = limit;

        state.page = await api.getOwnedProjects(state.cursor);
    }

    async function getDailyProjectData(payload: ProjectUsageDateRange): Promise<void> {
        const usage: ProjectsStorageBandwidthDaily = await api.getDailyUsage(state.selectedProject.id, payload.since, payload.before);

        state.allocatedBandwidthChartData = usage.allocatedBandwidth;
        state.storageChartData = usage.storage;
        state.chartDataSince = payload.since;
        state.chartDataBefore = payload.before;
    }

    async function createProject(createProjectFields: ProjectFields): Promise<string> {
        const createdProject = await api.create(createProjectFields);

        state.projects.push(createdProject);

        return createdProject.id;
    }

    async function createDefaultProject(userID: string): Promise<void> {
        const UNTITLED_PROJECT_NAME = 'My First Project';
        const UNTITLED_PROJECT_DESCRIPTION = '___';

        const project = new ProjectFields(
            UNTITLED_PROJECT_NAME,
            UNTITLED_PROJECT_DESCRIPTION,
            userID,
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

        state.currentLimits = readonly({
            ...state.currentLimits,
            storageLimit: limitsToUpdate.storageLimit,
        });
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

        state.currentLimits = readonly({
            ...state.currentLimits,
            bandwidthLimit: limitsToUpdate.bandwidthLimit,
        });
    }

    async function getProjectLimits(projectID: string): Promise<void> {
        state.currentLimits = await api.getLimits(projectID);
    }

    async function getTotalLimits(): Promise<void> {
        state.totalLimits = await api.getTotalLimits();
    }

    async function getProjectSalt(projectID: string): Promise<string> {
        return await api.getSalt(projectID);
    }

    async function getUserInvitations(): Promise<ProjectInvitation[]> {
        const invites = await api.getUserInvitations();

        state.invitations = invites;

        return invites;
    }

    async function respondToInvitation(projectID: string, response: ProjectInvitationResponse): Promise<void> {
        await api.respondToInvitation(projectID, response);
    }

    function selectInvitation(invite: ProjectInvitation): void {
        state.selectedInvitation = invite;
    }

    function clear(): void {
        state.projects = [];
        state.selectedProject = DEFAULT_PROJECT;
        state.currentLimits = DEFAULT_PROJECT_LIMITS;
        state.totalLimits = new ProjectLimits();
        state.storageChartData = [];
        state.allocatedBandwidthChartData = [];
        state.chartDataSince = new Date();
        state.chartDataBefore = new Date();
        state.invitations = [];
        state.selectedInvitation = DEFAULT_INVITATION;
    }

    function projectsCount(userID: string): number {
        let projectsCount = 0;

        state.projects.forEach((project: Project) => {
            if (project.ownerId === userID) {
                projectsCount++;
            }
        });

        return projectsCount;
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

    return {
        state,
        getProjects,
        getOwnedProjects,
        getDailyProjectData,
        createProject,
        createDefaultProject,
        selectProject,
        updateProjectName,
        updateProjectDescription,
        updateProjectStorageLimit,
        updateProjectBandwidthLimit,
        getProjectLimits,
        getTotalLimits,
        getProjectSalt,
        getUserInvitations,
        respondToInvitation,
        selectInvitation,
        projectsCount,
        clear,
        projects,
        projectsWithoutSelected,
    };
});
