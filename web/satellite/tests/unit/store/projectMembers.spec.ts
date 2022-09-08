// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue } from '@vue/test-utils';

import { ProjectMembersApiGql } from '@/api/projectMembers';
import { ProjectsApiGql } from '@/api/projects';
import { makeProjectMembersModule, PROJECT_MEMBER_MUTATIONS } from '@/store/modules/projectMembers';
import { makeProjectsModule } from '@/store/modules/projects';
import { SortDirection } from '@/types/common';
import { ProjectMember, ProjectMemberOrderBy, ProjectMembersPage } from '@/types/projectMembers';
import { Project } from '@/types/projects';
import { PM_ACTIONS } from '@/utils/constants/actionNames';

const projectsApi = new ProjectsApiGql();
const projectsModule = makeProjectsModule(projectsApi);
const selectedProject = new Project();
selectedProject.id = '1';
projectsModule.state.selectedProject = selectedProject;

const FIRST_PAGE = 1;
const TEST_ERROR = new Error('testError');
const UNREACHABLE_ERROR = 'should be unreachable';

const Vue = createLocalVue();
const pmApi = new ProjectMembersApiGql();
const projectMembersModule = makeProjectMembersModule(pmApi);

Vue.use(Vuex);

const store = new Vuex.Store<{
    projectsModule: typeof projectsModule.state,
    projectMembersModule: typeof projectMembersModule.state,
}>({ modules: { projectsModule, projectMembersModule } });
const state = store.state.projectMembersModule;
const date = new Date(0);
const projectMember1 = new ProjectMember('testFullName1', 'testShortName1', 'test1@example.com', date, '1');
const projectMember2 = new ProjectMember('testFullName2', 'testShortName2', 'test2@example.com', date, '2');

describe('mutations', () => {
    it('fetch project members', function () {
        const testProjectMembersPage = new ProjectMembersPage();
        testProjectMembersPage.projectMembers = [projectMember1];
        testProjectMembersPage.totalCount = 1;
        testProjectMembersPage.pageCount = 1;

        store.commit(PROJECT_MEMBER_MUTATIONS.FETCH, testProjectMembersPage);

        expect(state.page.projectMembers.length).toBe(1);
        expect(state.page.search).toBe('');
        expect(state.page.order).toBe(ProjectMemberOrderBy.NAME);
        expect(state.page.orderDirection).toBe(SortDirection.ASCENDING);
        expect(state.page.limit).toBe(6);
        expect(state.page.pageCount).toBe(1);
        expect(state.page.currentPage).toBe(1);
        expect(state.page.totalCount).toBe(1);
    });

    it('set project members page', function () {
        store.commit(PROJECT_MEMBER_MUTATIONS.SET_PAGE, 2);

        expect(state.cursor.page).toBe(2);
    });

    it('set search query', function () {
        store.commit(PROJECT_MEMBER_MUTATIONS.SET_SEARCH_QUERY, 'testSearchQuery');

        expect(state.cursor.search).toBe('testSearchQuery');
    });

    it('set sort order', function () {
        store.commit(PROJECT_MEMBER_MUTATIONS.CHANGE_SORT_ORDER, ProjectMemberOrderBy.EMAIL);

        expect(state.cursor.order).toBe(ProjectMemberOrderBy.EMAIL);
    });

    it('set sort direction', function () {
        store.commit(PROJECT_MEMBER_MUTATIONS.CHANGE_SORT_ORDER_DIRECTION, SortDirection.DESCENDING);

        expect(state.cursor.orderDirection).toBe(SortDirection.DESCENDING);
    });

    it('toggle selection', function () {
        const testProjectMembersPage = new ProjectMembersPage();
        testProjectMembersPage.projectMembers = [projectMember1];
        testProjectMembersPage.totalCount = 1;
        testProjectMembersPage.pageCount = 1;

        store.commit(PROJECT_MEMBER_MUTATIONS.TOGGLE_SELECTION, projectMember1);

        expect(state.page.projectMembers[0].isSelected).toBe(true);
        expect(state.selectedProjectMembersEmails.length).toBe(1);

        store.commit(PROJECT_MEMBER_MUTATIONS.FETCH, testProjectMembersPage);

        expect(state.selectedProjectMembersEmails.length).toBe(1);

        store.commit(PROJECT_MEMBER_MUTATIONS.TOGGLE_SELECTION, projectMember1);

        expect(state.page.projectMembers[0].isSelected).toBe(false);
        expect(state.selectedProjectMembersEmails.length).toBe(0);
    });

    it('clear selection', function () {
        store.commit(PROJECT_MEMBER_MUTATIONS.CLEAR_SELECTION);

        state.page.projectMembers.forEach((pm: ProjectMember) => {
            expect(pm.isSelected).toBe(false);
        });

        expect(state.selectedProjectMembersEmails.length).toBe(0);
    });

    it('clear store', function () {
        store.commit(PROJECT_MEMBER_MUTATIONS.CLEAR);

        expect(state.cursor.page).toBe(1);
        expect(state.cursor.search).toBe('');
        expect(state.cursor.order).toBe(ProjectMemberOrderBy.NAME);
        expect(state.cursor.orderDirection).toBe(SortDirection.ASCENDING);
        expect(state.page.projectMembers.length).toBe(0);
        expect(state.selectedProjectMembersEmails.length).toBe(0);
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
    });

    it('add project members', async function () {
        jest.spyOn(pmApi, 'add').mockReturnValue(Promise.resolve());

        try {
            await store.dispatch(PM_ACTIONS.ADD, [projectMember1.user.email]);
            throw TEST_ERROR;
        } catch (err) {
            expect(err).toBe(TEST_ERROR);
        }
    });

    it('add project member throws error when api call fails', async function () {
        jest.spyOn(pmApi, 'add').mockImplementation(() => {
            throw TEST_ERROR;
        });

        const stateDump = state;

        try {
            await store.dispatch(PM_ACTIONS.ADD, [projectMember1.user.email]);
        } catch (err) {
            expect(err).toBe(TEST_ERROR);
            expect(state).toBe(stateDump);

            return;
        }

        fail(UNREACHABLE_ERROR);
    });

    it('delete project members', async function () {
        jest.spyOn(pmApi, 'delete').mockReturnValue(Promise.resolve());

        try {
            await store.dispatch(PM_ACTIONS.DELETE, [projectMember1.user.email]);
            throw TEST_ERROR;
        } catch (err) {
            expect(err).toBe(TEST_ERROR);
        }
    });

    it('delete project member throws error when api call fails', async function () {
        jest.spyOn(pmApi, 'delete').mockImplementation(() => {
            throw TEST_ERROR;
        });

        const stateDump = state;

        try {
            await store.dispatch(PM_ACTIONS.DELETE, [projectMember1.user.email]);
        } catch (err) {
            expect(err).toBe(TEST_ERROR);
            expect(state).toBe(stateDump);

            return;
        }

        fail(UNREACHABLE_ERROR);
    });

    it('fetch project members', async function () {
        jest.spyOn(pmApi, 'get').mockReturnValue(
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

        await store.dispatch(PM_ACTIONS.FETCH, FIRST_PAGE);

        expect(state.page.projectMembers[0].isSelected).toBe(false);
        expect(state.page.projectMembers[0].joinedAt).toBe(projectMember1.joinedAt);
        expect(state.page.projectMembers[0].user.email).toBe(projectMember1.user.email);
        expect(state.page.projectMembers[0].user.id).toBe(projectMember1.user.id);
        expect(state.page.projectMembers[0].user.partnerId).toBe(projectMember1.user.partnerId);
        expect(state.page.projectMembers[0].user.fullName).toBe(projectMember1.user.fullName);
        expect(state.page.projectMembers[0].user.shortName).toBe(projectMember1.user.shortName);
    });

    it('fetch project members throws error when api call fails', async function () {
        jest.spyOn(pmApi, 'get').mockImplementation(() => {
            throw TEST_ERROR;
        });

        const stateDump = state;

        try {
            await store.dispatch(PM_ACTIONS.FETCH, FIRST_PAGE);
        } catch (err) {
            expect(err).toBe(TEST_ERROR);
            expect(state).toBe(stateDump);

            return;
        }

        fail(UNREACHABLE_ERROR);
    });

    it('set project members search query', function () {
        store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, 'search');

        expect(state.cursor.search).toBe('search');
    });

    it('set project members sort by', function () {
        store.dispatch(PM_ACTIONS.SET_SORT_BY, ProjectMemberOrderBy.CREATED_AT);

        expect(state.cursor.order).toBe(ProjectMemberOrderBy.CREATED_AT);
    });

    it('set sort direction', function () {
        store.dispatch(PM_ACTIONS.SET_SORT_DIRECTION, SortDirection.DESCENDING);

        expect(state.cursor.orderDirection).toBe(SortDirection.DESCENDING);
    });

    it('toggle selection', async function () {
        jest.spyOn(pmApi, 'get').mockReturnValue(
            Promise.resolve(new ProjectMembersPage(
                [projectMember1, projectMember2],
                '',
                ProjectMemberOrderBy.NAME,
                SortDirection.ASCENDING,
                6,
                1,
                1,
                2)),
        );

        await store.dispatch(PM_ACTIONS.FETCH, FIRST_PAGE);
        store.dispatch(PM_ACTIONS.TOGGLE_SELECTION, projectMember1);

        expect(state.page.projectMembers[0].isSelected).toBe(true);
        expect(state.selectedProjectMembersEmails.length).toBe(1);

        store.dispatch(PM_ACTIONS.TOGGLE_SELECTION, projectMember2);

        expect(state.page.projectMembers[1].isSelected).toBe(true);
        expect(state.selectedProjectMembersEmails.length).toBe(2);

        await store.dispatch(PM_ACTIONS.FETCH, FIRST_PAGE);

        expect(state.page.projectMembers[1].isSelected).toBe(true);
        expect(state.selectedProjectMembersEmails.length).toBe(2);

        store.dispatch(PM_ACTIONS.TOGGLE_SELECTION, projectMember1);

        expect(state.page.projectMembers[0].isSelected).toBe(false);
        expect(state.selectedProjectMembersEmails.length).toBe(1);
    });

    it('clear selection', function () {
        store.dispatch(PM_ACTIONS.CLEAR_SELECTION);

        state.page.projectMembers.forEach((pm: ProjectMember) => {
            expect(pm.isSelected).toBe(false);
        });
    });

    it('clear store', function () {
        store.commit(PM_ACTIONS.CLEAR);

        expect(state.cursor.page).toBe(1);
        expect(state.cursor.search).toBe('');
        expect(state.cursor.order).toBe(ProjectMemberOrderBy.NAME);
        expect(state.cursor.orderDirection).toBe(SortDirection.ASCENDING);
        expect(state.page.projectMembers.length).toBe(0);

        state.page.projectMembers.forEach((pm: ProjectMember) => {
            expect(pm.isSelected).toBe(false);
        });
    });
});

describe('getters', () => {
    const selectedProjectMember = new ProjectMember('testFullName2', 'testShortName2', 'test2@example.com', date, '2');

    it('selected project members', function () {
        const testProjectMembersPage = new ProjectMembersPage();
        testProjectMembersPage.projectMembers = [selectedProjectMember];
        testProjectMembersPage.totalCount = 1;
        testProjectMembersPage.pageCount = 1;

        store.commit(PROJECT_MEMBER_MUTATIONS.FETCH, testProjectMembersPage);
        store.commit(PROJECT_MEMBER_MUTATIONS.TOGGLE_SELECTION, selectedProjectMember);

        const retrievedProjectMembers = store.getters.selectedProjectMembers;

        expect(retrievedProjectMembers[0].user.id).toBe(selectedProjectMember.user.id);
    });
});
