// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { describe, beforeEach, it, expect, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';

import { ProjectsHttpApi } from '@/api/projects';
import { Project, ProjectFields, ProjectLimits } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { randomUUID } from '@/utils/idGenerator';

const limits = new ProjectLimits(null, null, 15, 12, 14, 13);
const project = new Project(randomUUID(), 'name', 'descr', '23', 'testOwnerId');
const projects = [
    new Project(
        randomUUID(),
        'name',
        'descr',
        '23',
        'testOwnerId',
        false,
    ),
    new Project(
        randomUUID(),
        'name2',
        'descr2',
        '24',
        'testOwnerId1',
        false,
    ),
];

describe('actions', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
        vi.resetAllMocks();
    });

    it('select project', () => {
        const store = useProjectsStore();

        store.state.projects = projects;
        store.selectProject(projects[0].id);

        expect(store.state.selectedProject.id).toBe(projects[0].id);
        expect(store.state.currentLimits.bandwidthLimit).toBe(0);
    });

    it('success fetch projects', async () => {
        const store = useProjectsStore();

        vi.spyOn(ProjectsHttpApi.prototype, 'get').mockReturnValue(
            Promise.resolve(projects),
        );

        await store.getProjects();

        expect(store.state.projects).toStrictEqual(projects);
    });

    it('fetch throws an error when api call fails', async () => {
        const store = useProjectsStore();

        vi.spyOn(ProjectsHttpApi.prototype, 'get').mockImplementation(() => { throw new Error(); });

        try {
            await store.getProjects();
        } catch (error) {
            expect(store.state.projects.length).toBe(0);
            expect(store.state.currentLimits.bandwidthLimit).toBe(0);
        }
    });

    it('success create project', async () => {
        const store = useProjectsStore();

        vi.spyOn(ProjectsHttpApi.prototype, 'create').mockReturnValue(
            Promise.resolve(project),
        );

        await store.createProject(new ProjectFields());

        expect(store.state.projects.length).toBe(1);
        expect(store.state.currentLimits.bandwidthLimit).toBe(0);
    });

    it('create throws an error when create api call fails', async () => {
        const store = useProjectsStore();

        vi.spyOn(ProjectsHttpApi.prototype, 'create').mockImplementation(() => { throw new Error(); });

        try {
            await store.createProject(new ProjectFields());
            expect(true).toBe(false);
        } catch (error) {
            expect(store.state.projects.length).toBe(0);
            expect(store.state.currentLimits.bandwidthLimit).toBe(0);
        }
    });

    it('success update project name', async () => {
        const store = useProjectsStore();

        vi.spyOn(ProjectsHttpApi.prototype, 'update').mockReturnValue(
            Promise.resolve(),
        );

        store.state.selectedProject = projects[0];
        const newName = 'newName';
        const fieldsToUpdate = new ProjectFields(newName, projects[0].description);

        await store.updateProjectName(fieldsToUpdate);

        expect(store.state.selectedProject.name).toBe(newName);
    });

    it('success update project description', async () => {
        const store = useProjectsStore();

        vi.spyOn(ProjectsHttpApi.prototype, 'update').mockReturnValue(
            Promise.resolve(),
        );

        store.state.selectedProject = projects[0];
        const newDescription = 'newDescription1';
        const fieldsToUpdate = new ProjectFields(projects[0].name, newDescription);

        await store.updateProjectDescription(fieldsToUpdate);

        expect(store.state.selectedProject.description).toBe(newDescription);
    });

    it('success get project limits', async () => {
        const store = useProjectsStore();

        vi.spyOn(ProjectsHttpApi.prototype, 'getLimits').mockReturnValue(
            Promise.resolve(limits),
        );

        store.state.projects = projects;

        await store.getProjectLimits(store.state.selectedProject.id);

        expect(store.state.currentLimits.bandwidthUsed).toBe(12);
        expect(store.state.currentLimits.bandwidthLimit).toBe(15);
        expect(store.state.currentLimits.storageUsed).toBe(13);
        expect(store.state.currentLimits.storageLimit).toBe(14);
    });
});

describe('getters', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
    });

    it('projects array', () => {
        const store = useProjectsStore();

        store.state.projects = projects;

        const allProjects = store.projects;

        expect(allProjects.length).toEqual(2);
    });
});
