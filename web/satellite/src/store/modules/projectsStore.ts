// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, ComputedRef, reactive, readonly } from 'vue';

import {
    DataStamp,
    LimitToChange,
    Project,
    ProjectFields,
    ProjectLimits,
    ProjectsApi,
    ProjectsStorageBandwidthDaily,
    ProjectUsageDateRange,
    ProjectInvitation,
    ProjectInvitationResponse,
    Emission,
    ProjectConfig,
    ProjectDeletionData,
} from '@/types/projects';
import { ProjectsHttpApi } from '@/api/projects';
import { hexToBase64 } from '@/utils/strings';
import { Duration, Time } from '@/utils/time';
import { useConfigStore } from '@/store/modules/configStore';
import { DeleteProjectStep } from '@/types/accountActions';

const DEFAULT_PROJECT = new Project('', '', '', '', '', 0);
const DEFAULT_INVITATION = new ProjectInvitation('', '', '', '', new Date());
const MAXIMUM_URL_ID_LENGTH = 22; // UUID (16 bytes) is 22 base64 characters

export const MINIMUM_URL_ID_LENGTH = 11;
export const DEFAULT_PROJECT_LIMITS = readonly(new ProjectLimits());

export class ProjectsState {
    public projects: Project[] = [];
    public selectedProject: Project = DEFAULT_PROJECT;
    public selectedProjectConfig: ProjectConfig = new ProjectConfig();
    public currentLimits: Readonly<ProjectLimits> = DEFAULT_PROJECT_LIMITS;
    public totalLimits: Readonly<ProjectLimits> = DEFAULT_PROJECT_LIMITS;
    public settledBandwidthChartData: DataStamp[] = [];
    public storageChartData: DataStamp[] = [];
    public chartDataSince: Date = new Date();
    public chartDataBefore: Date = new Date();
    public invitations: ProjectInvitation[] = [];
    public selectedInvitation: ProjectInvitation = DEFAULT_INVITATION;
    public emission: Emission = new Emission();
}

export const useProjectsStore = defineStore('projects', () => {
    const state = reactive<ProjectsState>(new ProjectsState());

    const api: ProjectsApi = new ProjectsHttpApi();

    const configStore = useConfigStore();
    const csrfToken = computed<string>(() => configStore.state.config.csrfToken);

    const selectedProjectConfig: ComputedRef<ProjectConfig> = computed(() => state.selectedProjectConfig);

    const usersFirstProject = computed<Project>(() => {
        return state.projects.reduce((earliest, current) => {
            return new Date(current.createdAt).getTime() < new Date(earliest.createdAt).getTime() ? current : earliest;
        }, state.projects[0]);
    });

    function getUsageReportLink(startUTC: Date, endUTC: Date, includeCost: boolean, projectSummary: boolean, projectID = ''): string {
        const since = Time.toUnixTimestamp(startUTC);
        const before = Time.toUnixTimestamp(endUTC);

        const allowedDuration = new Duration(useConfigStore().state.config.allowedUsageReportDateRange);
        const duration = before - since; // seconds
        if (duration > allowedDuration.fullSeconds) {
            throw new Error(`Date range must be less than ${allowedDuration.shortString}`);
        }

        return api.getTotalUsageReportLink(since, before, includeCost, projectSummary, projectID);
    }

    async function getProjects(): Promise<Project[]> {
        const projects = await api.get();

        setProjects(projects);

        return projects;
    }

    async function deleteProject(projectId: string, step: DeleteProjectStep, data: string): Promise<ProjectDeletionData | null> {
        const resp = await api.delete(projectId, step, data, csrfToken.value);
        if (!resp && step === DeleteProjectStep.ConfirmDeleteStep) {
            state.projects = state.projects.filter((p) => p.id !== projectId);
        }
        return resp;
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

    async function getDailyProjectData(payload: ProjectUsageDateRange): Promise<void> {
        const usage: ProjectsStorageBandwidthDaily = await api.getDailyUsage(state.selectedProject.id, payload.since, payload.before);

        state.settledBandwidthChartData = usage.settledBandwidth;
        state.storageChartData = usage.storage;
        state.chartDataSince = payload.since;
        state.chartDataBefore = payload.before;
    }

    async function createProject(createProjectFields: ProjectFields): Promise<Project> {
        const createdProject = await api.create(createProjectFields, csrfToken.value);

        state.projects.push(createdProject);
        calculateURLIds();

        return createdProject;
    }

    async function createDefaultProject(userID: string, managePassphrase = false): Promise<void> {
        const UNTITLED_PROJECT_NAME = `My ${configStore.isDefaultBrand ? 'Storj ' : ''}Project`;
        const UNTITLED_PROJECT_DESCRIPTION = '___';

        const project = new ProjectFields(
            UNTITLED_PROJECT_NAME,
            UNTITLED_PROJECT_DESCRIPTION,
            userID,
            managePassphrase,
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

    function deselectProject(): void {
        state.selectedProject = DEFAULT_PROJECT;
    }

    async function getProjectConfig(): Promise<ProjectConfig> {
        state.selectedProjectConfig = await api.getConfig(state.selectedProject.id);
        return state.selectedProjectConfig;
    }

    async function updateProjectName(fieldsToUpdate: ProjectFields): Promise<void> {
        await api.update(state.selectedProject.id, {
            name: fieldsToUpdate.name,
            description: state.selectedProject.description,
        }, csrfToken.value);

        state.selectedProject.name = fieldsToUpdate.name;
    }

    async function updateProjectDescription(fieldsToUpdate: ProjectFields): Promise<void> {
        await api.update(state.selectedProject.id, {
            name: state.selectedProject.name,
            description: fieldsToUpdate.description,
        }, csrfToken.value);

        state.selectedProject.description = fieldsToUpdate.description;
    }

    async function updateProjectStorageLimit(newLimit: number): Promise<void> {
        await api.updateLimits(state.selectedProject.id, {
            storageLimit: newLimit.toString(),
        }, csrfToken.value);

        state.currentLimits = readonly({
            ...state.currentLimits,
            userSetStorageLimit: newLimit,
        });
    }

    async function updateProjectBandwidthLimit(newLimit: number): Promise<void> {
        await api.updateLimits(state.selectedProject.id, {
            bandwidthLimit: newLimit.toString(),
        }, csrfToken.value);

        state.currentLimits = readonly({
            ...state.currentLimits,
            userSetBandwidthLimit: newLimit,
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

    async function migratePricing(projectID: string): Promise<void> {
        await api.migratePricing(projectID, csrfToken.value);
    }

    async function getProjectLimits(projectID: string): Promise<void> {
        state.currentLimits = await api.getLimits(projectID);
    }

    async function getTotalLimits(): Promise<void> {
        state.totalLimits = await api.getTotalLimits();
    }

    async function getProjectSalt(projectID: string): Promise<string> {
        return state.selectedProjectConfig.salt || await api.getSalt(projectID);
    }

    async function getEmissionImpact(projectID: string): Promise<void> {
        state.emission = await api.getEmissionImpact(projectID);
    }

    async function getUserInvitations(): Promise<ProjectInvitation[]> {
        const invites = await api.getUserInvitations();

        state.invitations = invites;

        return invites;
    }

    async function respondToInvitation(projectID: string, response: ProjectInvitationResponse): Promise<void> {
        await api.respondToInvitation(projectID, response, csrfToken.value);
    }

    function clear(): void {
        state.projects = [];
        state.selectedProject = DEFAULT_PROJECT;
        state.currentLimits = DEFAULT_PROJECT_LIMITS;
        state.totalLimits = new ProjectLimits();
        state.storageChartData = [];
        state.settledBandwidthChartData = [];
        state.chartDataSince = new Date();
        state.chartDataBefore = new Date();
        state.invitations = [];
        state.selectedInvitation = DEFAULT_INVITATION;
    }

    const projects = computed(() => {
        return state.projects;
    });

    const projectsWithoutSelected = computed(() => {
        return state.projects.filter((project: Project) => {
            return project.id !== state.selectedProject.id;
        });
    });

    return {
        state,
        selectedProjectConfig,
        usersFirstProject,
        getProjects,
        deleteProject,
        getDailyProjectData,
        createProject,
        createDefaultProject,
        selectProject,
        deselectProject,
        getProjectConfig,
        updateProjectName,
        updateProjectDescription,
        updateProjectStorageLimit,
        updateProjectBandwidthLimit,
        requestLimitIncrease,
        getProjectLimits,
        getTotalLimits,
        getUsageReportLink,
        getProjectSalt,
        getEmissionImpact,
        getUserInvitations,
        respondToInvitation,
        migratePricing,
        clear,
        projects,
        projectsWithoutSelected,
    };
});
