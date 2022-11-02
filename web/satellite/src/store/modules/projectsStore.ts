// Copyright (C) 2019 Storj Labs, Inc.
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
    const projectsStore = reactive<ProjectsState>({
        projects: [],
        selectedProject: defaultSelectedProject,
        currentLimits: new ProjectLimits(),
        totalLimits: new ProjectLimits(),
        cursor: new ProjectsCursor(),
        page: new ProjectsPage(),
        allocatedBandwidthChartData: [],
        settledBandwidthChartData: [],
        storageChartData: [],
        chartDataSince: new Date(),
        chartDataBefore: new Date(),
    });

    const api: ProjectsApi = new ProjectsApiGql();

    async function fetchProjects(): Promise<Project[]> {
        const projects = await api.get();

        setProjects(projects);

        return projects;
    }

    function setProjects(projects: Project[]): void {
        projectsStore.projects = projects;

        if (!projectsStore.selectedProject.id) {
            return;
        }

        const projectsCount = projectsStore.projects.length;

        for (let i = 0; i < projectsCount; i++) {
            const project = projectsStore.projects[i];

            if (project.id !== projectsStore.selectedProject.id) {
                continue;
            }

            projectsStore.selectedProject = project;

            return;
        }

        projectsStore.selectedProject = defaultSelectedProject;
    }

    async function fetchOwnedProjects(pageNumber: number): Promise<void> {
        projectsStore.cursor.page = pageNumber;
        projectsStore.cursor.limit = PROJECT_PAGE_LIMIT;

        projectsStore.page = await api.getOwnedProjects(projectsStore.cursor);
    }

    async function fetchDailyProjectData(payload: ProjectUsageDateRange): Promise<void> {
        const usage: ProjectsStorageBandwidthDaily = await api.getDailyUsage(projectsStore.selectedProject.id, payload.since, payload.before);

        projectsStore.allocatedBandwidthChartData = usage.allocatedBandwidth;
        projectsStore.settledBandwidthChartData = usage.settledBandwidth;
        projectsStore.storageChartData = usage.storage;
        projectsStore.chartDataSince = payload.since;
        projectsStore.chartDataBefore = payload.before;
    }

    async function createProject(createProjectFields: ProjectFields): Promise<string> {
        const createdProject = await api.create(createProjectFields);

        projectsStore.projects.push(createdProject);

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
        const selected = projectsStore.projects.find((project: Project) => project.id === projectID);

        if (!selected) {
            return;
        }

        projectsStore.selectedProject = selected;
    }

    async function updateProjectName(fieldsToUpdate: ProjectFields): Promise<void> {
        const project = new ProjectFields(
            fieldsToUpdate.name,
            projectsStore.selectedProject.description,
            projectsStore.selectedProject.id,
        );
        const limit = new ProjectLimits(
            projectsStore.currentLimits.bandwidthLimit,
            projectsStore.currentLimits.bandwidthUsed,
            projectsStore.currentLimits.storageLimit,
            projectsStore.currentLimits.storageUsed,
        );

        await api.update(projectsStore.selectedProject.id, project, limit);

        projectsStore.selectedProject.name = fieldsToUpdate.name;
    }

    async function updateProjectDescription(fieldsToUpdate: ProjectFields): Promise<void> {
        const project = new ProjectFields(
            projectsStore.selectedProject.name,
            fieldsToUpdate.description,
            projectsStore.selectedProject.id,
        );
        const limit = new ProjectLimits(
            projectsStore.currentLimits.bandwidthLimit,
            projectsStore.currentLimits.bandwidthUsed,
            projectsStore.currentLimits.storageLimit,
            projectsStore.currentLimits.storageUsed,
        );
        await api.update(projectsStore.selectedProject.id, project, limit);

        projectsStore.selectedProject.description = fieldsToUpdate.description;
    }

    async function updateProjectStorageLimit(limitsToUpdate: ProjectLimits): Promise<void> {
        const project = new ProjectFields(
            projectsStore.selectedProject.name,
            projectsStore.selectedProject.description,
            projectsStore.selectedProject.id,
        );
        const limit = new ProjectLimits(
            projectsStore.currentLimits.bandwidthLimit,
            projectsStore.currentLimits.bandwidthUsed,
            limitsToUpdate.storageLimit,
            projectsStore.currentLimits.storageUsed,
        );
        await api.update(projectsStore.selectedProject.id, project, limit);

        projectsStore.currentLimits.storageLimit = limitsToUpdate.storageLimit;
    }

    async function updateProjectBandwidthLimit(limitsToUpdate: ProjectLimits): Promise<void> {
        const project = new ProjectFields(
            projectsStore.selectedProject.name,
            projectsStore.selectedProject.description,
            projectsStore.selectedProject.id,
        );
        const limit = new ProjectLimits(
            limitsToUpdate.bandwidthLimit,
            projectsStore.currentLimits.bandwidthUsed,
            projectsStore.currentLimits.storageLimit,
            projectsStore.currentLimits.storageUsed,
        );
        await api.update(projectsStore.selectedProject.id, project, limit);

        projectsStore.currentLimits.bandwidthLimit = limitsToUpdate.bandwidthLimit;
    }

    async function deleteProject(projectID: string): Promise<void> {
        await api.delete(projectID);

        projectsStore.projects = projectsStore.projects.filter(project => project.id !== projectID);

        if (projectsStore.selectedProject.id === projectID) {
            projectsStore.selectedProject = new Project();
        }
    }

    async function fetchProjectLimits(projectID: string): Promise<void> {
        projectsStore.currentLimits = await api.getLimits(projectID);
    }

    async function fetchTotalLimits(): Promise<void> {
        projectsStore.totalLimits = await api.getTotalLimits();
    }

    async function getProjectSalt(projectID: string): Promise<string> {
        return await api.getSalt(projectID);
    }

    function clearProjectState(): void {
        projectsStore.projects = [];
        projectsStore.selectedProject = defaultSelectedProject;
        projectsStore.currentLimits = new ProjectLimits();
        projectsStore.totalLimits = new ProjectLimits();
        projectsStore.storageChartData = [];
        projectsStore.allocatedBandwidthChartData = [];
        projectsStore.settledBandwidthChartData = [];
        projectsStore.chartDataSince = new Date();
        projectsStore.chartDataBefore = new Date();
    }

    const projects = computed(() => {
        return projectsStore.projects.map((project: Project) => {
            if (project.id === projectsStore.selectedProject.id) {
                project.isSelected = true;
            }

            return project;
        });
    });

    const projectsWithoutSelected = computed(() => {
        return projectsStore.projects.filter((project: Project) => {
            return project.id !== projectsStore.selectedProject.id;
        });
    });

    const projectsCount = computed(() => {
        let projectsCount = 0;

        const { usersState } = useUsersStore();

        projectsStore.projects.forEach((project: Project) => {
            if (project.ownerId === usersState.user.id) {
                projectsCount++;
            }
        });

        return projectsCount;
    });

    return {
        projectsStore,
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
    };
});
