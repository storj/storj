// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';

import { ProjectMembersApiGql } from '@/api/projectMembers';
import { SortDirection } from '@/types/common';
import { ProjectMember, ProjectMemberOrderBy, ProjectMembersPage } from '@/types/projectMembers';
import { Project } from '@/types/projects';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';

const selectedProject = new Project();
selectedProject.id = '1';

const FIRST_PAGE = 1;
const TEST_ERROR = new Error('testError');
const UNREACHABLE_ERROR = 'should be unreachable';

const date = new Date(0);
const projectMember1 = new ProjectMember('testFullName1', 'testShortName1', 'test1@example.com', date, '1');
const projectMember2 = new ProjectMember('testFullName2', 'testShortName2', 'test2@example.com', date, '2');

describe('actions', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
        vi.resetAllMocks();
    });

    it('fetch project members', async function () {
        const store = useProjectMembersStore();

        const testProjectMembersPage = new ProjectMembersPage();
        testProjectMembersPage.projectMembers = [projectMember1];
        testProjectMembersPage.totalCount = 1;
        testProjectMembersPage.pageCount = 1;

        vi.spyOn(ProjectMembersApiGql.prototype, 'get')
            .mockImplementation(() => Promise.resolve(testProjectMembersPage));

        await store.getProjectMembers(FIRST_PAGE, selectedProject.id);

        expect(store.state.page.projectMembers.length).toBe(1);
        expect(store.state.page.search).toBe('');
        expect(store.state.page.order).toBe(ProjectMemberOrderBy.NAME);
        expect(store.state.page.orderDirection).toBe(SortDirection.ASCENDING);
        expect(store.state.page.limit).toBe(6);
        expect(store.state.page.pageCount).toBe(1);
        expect(store.state.page.currentPage).toBe(1);
        expect(store.state.page.totalCount).toBe(1);
    });

    it('set project members page number', function () {
        const store = useProjectMembersStore();

        store.setPageNumber(2);

        expect(store.state.cursor.page).toBe(2);
    });

    it('set search query', function () {
        const store = useProjectMembersStore();

        store.setSearchQuery('testSearchQuery');

        expect(store.state.cursor.search).toBe('testSearchQuery');
    });

    it('set sort order', function () {
        const store = useProjectMembersStore();

        store.setSortingBy(ProjectMemberOrderBy.EMAIL);

        expect(store.state.cursor.order).toBe(ProjectMemberOrderBy.EMAIL);
    });

    it('set sort direction', function () {
        const store = useProjectMembersStore();

        store.setSortingDirection(SortDirection.DESCENDING);

        expect(store.state.cursor.orderDirection).toBe(SortDirection.DESCENDING);
    });

    it('toggle selection', async function () {
        const store = useProjectMembersStore();

        const testProjectMembersPage = new ProjectMembersPage();
        testProjectMembersPage.projectMembers = [projectMember1];
        testProjectMembersPage.totalCount = 1;
        testProjectMembersPage.pageCount = 1;

        store.setPage(testProjectMembersPage);
        store.toggleProjectMemberSelection(projectMember1);

        expect(store.state.page.projectMembers[0].isSelected).toBe(true);
        expect(store.state.selectedProjectMembersEmails.length).toBe(1);

        vi.spyOn(ProjectMembersApiGql.prototype, 'get')
            .mockImplementation(() => Promise.resolve(testProjectMembersPage));

        await store.getProjectMembers(FIRST_PAGE, selectedProject.id);

        expect(store.state.selectedProjectMembersEmails.length).toBe(1);

        store.toggleProjectMemberSelection(projectMember1);

        expect(store.state.page.projectMembers[0].isSelected).toBe(false);
        expect(store.state.selectedProjectMembersEmails.length).toBe(0);
    });

    it('clear selection', function () {
        const store = useProjectMembersStore();

        const testProjectMembersPage = new ProjectMembersPage();
        testProjectMembersPage.projectMembers = [projectMember1, projectMember2];
        testProjectMembersPage.totalCount = 1;
        testProjectMembersPage.pageCount = 1;

        store.setPage(testProjectMembersPage);
        store.toggleProjectMemberSelection(projectMember1);
        store.clearProjectMemberSelection();

        store.state.page.projectMembers.forEach((pm: ProjectMember) => {
            expect(pm.isSelected).toBe(false);
        });

        expect(store.state.selectedProjectMembersEmails.length).toBe(0);
    });

    it('add project members', async function () {
        const store = useProjectMembersStore();

        vi.spyOn(ProjectMembersApiGql.prototype, 'add').mockReturnValue(Promise.resolve());

        try {
            await store.addProjectMembers([projectMember1.user.email], selectedProject.id);
            throw TEST_ERROR;
        } catch (err) {
            expect(err).toBe(TEST_ERROR);
        }
    });

    it('add project member throws error when api call fails', async function () {
        const store = useProjectMembersStore();

        vi.spyOn(ProjectMembersApiGql.prototype, 'add').mockImplementation(() => {
            throw TEST_ERROR;
        });

        const stateDump = store.state;

        try {
            await store.addProjectMembers([projectMember1.user.email], selectedProject.id);
        } catch (err) {
            expect(err).toBe(TEST_ERROR);
            expect(store.state).toBe(stateDump);

            return;
        }

        fail(UNREACHABLE_ERROR);
    });

    it('delete project members', async function () {
        const store = useProjectMembersStore();

        vi.spyOn(ProjectMembersApiGql.prototype, 'delete').mockReturnValue(Promise.resolve());

        try {
            await store.deleteProjectMembers(selectedProject.id);
            throw TEST_ERROR;
        } catch (err) {
            expect(err).toBe(TEST_ERROR);
        }
    });

    it('delete project member throws error when api call fails', async function () {
        const store = useProjectMembersStore();

        vi.spyOn(ProjectMembersApiGql.prototype, 'delete').mockImplementation(() => {
            throw TEST_ERROR;
        });

        const stateDump = store.state;

        try {
            await store.deleteProjectMembers(selectedProject.id);
        } catch (err) {
            expect(err).toBe(TEST_ERROR);
            expect(store.state).toBe(stateDump);

            return;
        }

        fail(UNREACHABLE_ERROR);
    });

    it('fetch project members', async function () {
        const store = useProjectMembersStore();

        vi.spyOn(ProjectMembersApiGql.prototype, 'get').mockReturnValue(
            Promise.resolve(new ProjectMembersPage(
                [projectMember1],
                '',
                ProjectMemberOrderBy.NAME,
                SortDirection.ASCENDING,
                6,
                1,
                1,
                1)),
        );

        await store.getProjectMembers(FIRST_PAGE, selectedProject.id);

        expect(store.state.page.projectMembers[0].isSelected).toBe(false);
        expect(store.state.page.projectMembers[0].joinedAt).toBe(projectMember1.joinedAt);
        expect(store.state.page.projectMembers[0].user.email).toBe(projectMember1.user.email);
        expect(store.state.page.projectMembers[0].user.id).toBe(projectMember1.user.id);
        expect(store.state.page.projectMembers[0].user.fullName).toBe(projectMember1.user.fullName);
        expect(store.state.page.projectMembers[0].user.shortName).toBe(projectMember1.user.shortName);
    });

    it('fetch project members throws error when api call fails', async function () {
        const store = useProjectMembersStore();

        vi.spyOn(ProjectMembersApiGql.prototype, 'get').mockImplementation(() => {
            throw TEST_ERROR;
        });

        const stateDump = store.state;

        try {
            await store.getProjectMembers(FIRST_PAGE, selectedProject.id);
        } catch (err) {
            expect(err).toBe(TEST_ERROR);
            expect(store.state).toBe(stateDump);

            return;
        }

        fail(UNREACHABLE_ERROR);
    });
});
