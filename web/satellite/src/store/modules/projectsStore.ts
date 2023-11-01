// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, reactive, readonly } from 'vue';

import {
    DataStamp,
    LimitToChange,
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
import { hexToBase64 } from '@/utils/strings';

const DEFAULT_PROJECT = new Project('', '', '', '', '', true, 0);
const DEFAULT_INVITATION = new ProjectInvitation('', '', '', '', new Date());
const MAXIMUM_URL_ID_LENGTH = 22; // UUID (16 bytes) is 22 base64 characters

export const MINIMUM_URL_ID_LENGTH = 11;
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

    function getUsageReportLink(projectID = ''): string {
        const now = new Date();
        const endUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), now.getUTCMinutes()));
        const startUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1, 0, 0));

        return api.getTotalUsageReportLink(startUTC, endUTC, projectID);
    }

    async function getProjects(): Promise<Project[]> {
        const projects = await api.get();

        setProjects(projects);

        return projects;
    }

    function calculateURLIds(): void {
        type urlIdInfo = {
            project: Project;
            base64Id: string;
            urlIdLength: number;
        };

        const occupied: Record<string, urlIdInfo[]> = {};

        state.projects.forEach(p => {
            const b64Id = hexToBase64(p.id.replaceAll('-', ''));
            const info: urlIdInfo = {
                project: p,
                base64Id: b64Id,
                urlIdLength: MINIMUM_URL_ID_LENGTH,
            };

            for (; info.urlIdLength <= MAXIMUM_URL_ID_LENGTH; info.urlIdLength++) {
                const urlId = b64Id.substring(0, info.urlIdLength);
                const others = occupied[urlId];

                if (others) {
                    if (info.urlIdLength === others[0].urlIdLength && info.urlIdLength !== MAXIMUM_URL_ID_LENGTH) {
                        others.forEach(other => {
                            occupied[other.base64Id.substring(0, ++other.urlIdLength)] = [other];
                        });
                    }
                    others.push(info);
                } else {
                    occupied[urlId] = [info];
                    break;
                }
            }
        });

        Object.keys(occupied).forEach(urlId => {
            const infos = occupied[urlId];
            if (infos.length !== 1) return;
            infos[0].project.urlId = urlId;
        });
    }

    function setProjects(projects: Project[]): void {
        state.projects = projects;
        calculateURLIds();

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

    async function createProject(createProjectFields: ProjectFields): Promise<Project> {
        const createdProject = await api.create(createProjectFields);

        state.projects.push(createdProject);
        calculateURLIds();

        return createdProject;
    }

    async function createDefaultProject(userID: string): Promise<void> {
        const UNTITLED_PROJECT_NAME = 'My First Project';
        const UNTITLED_PROJECT_DESCRIPTION = '___';

        const project = new ProjectFields(
            UNTITLED_PROJECT_NAME,
            UNTITLED_PROJECT_DESCRIPTION,
            userID,
        );

        const createdProject = await createProject(project);

        selectProject(createdProject.id);
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

    async function requestLimitIncrease(limitToRequest: LimitToChange, limit: number): Promise<void> {
        let curLimit = state.currentLimits.bandwidthLimit.toString();
        if (limitToRequest === LimitToChange.Storage) {
            curLimit = state.currentLimits.storageLimit.toString();
        }
        await api.requestLimitIncrease(state.selectedProject.id, {
            limitType: limitToRequest,
            currentLimit: curLimit,
            desiredLimit: limit.toString(),
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
        requestLimitIncrease,
        getProjectLimits,
        getTotalLimits,
        getUsageReportLink,
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
