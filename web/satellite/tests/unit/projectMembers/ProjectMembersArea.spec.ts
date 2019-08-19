// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, mount, shallowMount } from '@vue/test-utils';
import Vuex from 'vuex';
import ProjectMembersArea from '@/components/team/ProjectMembersArea.vue';
import { projectMembersModule } from '@/store/modules/projectMembers';
import { PROJECT_MEMBER_MUTATIONS } from '@/store/mutationConstants';
import { ProjectMember, ProjectMembersPage } from '@/types/projectMembers';

const localVue = createLocalVue();

localVue.use(Vuex);

const projectMember1 = new ProjectMember('testFullName1', 'testShortName1', 'test1@example.com', 'now1', '1');
const projectMember2 = new ProjectMember('testFullName2', 'testShortName2', 'test2@example.com', 'now2', '2');

describe('ProjectMembersArea.vue', () => {
    const state = projectMembersModule.state;
    const mutations = projectMembersModule.mutations;
    const actions = projectMembersModule.actions;
    const getters = projectMembersModule.getters;

    const store = new Vuex.Store({
        modules: {
            projectMembersModule: {
                state,
                mutations,
                actions,
                getters,
            }
        }
    });


    it('renders correctly', () => {
        const wrapper = shallowMount(ProjectMembersArea, {
            store,
            localVue
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('function fetchProjectMembers works correctly', () => {
        const testProjectMembersPage = new ProjectMembersPage();
        testProjectMembersPage.projectMembers = [projectMember1];
        testProjectMembersPage.totalCount = 1;
        testProjectMembersPage.pageCount = 1;

        store.commit(PROJECT_MEMBER_MUTATIONS.FETCH, testProjectMembersPage);

        const wrapper = mount(ProjectMembersArea, {
            store,
            localVue,
            mocks: {
                $route: {
                    query: {
                        pageNumber: null
                    }
                },
                $router: {
                    replace: () => false
                }
            }
        });

        expect(wrapper.vm.projectMembers.length).toBe(1);
    });

    it('action on toggle works correctly', () => {
        const wrapper = mount(ProjectMembersArea, {
            store,
            localVue,
            mocks: {
                $route: {
                    query: {}
                }
            }
        });

        wrapper.vm.onMemberClick(projectMember1);

        expect(store.getters.selectedProjectMembers.length).toBe(1);
    });

    it('clear selection works correctly', () => {
        const wrapper = mount(ProjectMembersArea, {
            store,
            localVue,
            mocks: {
                $route: {
                    query: {}
                }
            }
        });

        wrapper.vm.onMemberClick(projectMember1);

        expect(store.getters.selectedProjectMembers.length).toBe(0);
    });

    it('Reversing list order triggers rerender', () => {
        const testProjectMembersPage = new ProjectMembersPage();
        testProjectMembersPage.projectMembers = [projectMember1, projectMember2];
        testProjectMembersPage.totalCount = 2;
        testProjectMembersPage.pageCount = 1;

        store.commit(PROJECT_MEMBER_MUTATIONS.FETCH, testProjectMembersPage);

        const wrapper = mount(ProjectMembersArea, {
            store,
            localVue,
            mocks: {
                $route: {
                    query: {}
                }
            }
        });

        expect(wrapper.vm.projectMembers[0].user.id).toBe(projectMember1.user.id);

        testProjectMembersPage.projectMembers = [projectMember2, projectMember1];

        store.commit(PROJECT_MEMBER_MUTATIONS.FETCH, testProjectMembersPage);

        expect(wrapper.vm.projectMembers[0].user.id).toBe(projectMember2.user.id);
    });

    it('project member deletion trigger list rerender', () => {
        const testProjectMembersPage = new ProjectMembersPage();
        testProjectMembersPage.projectMembers = [projectMember1];
        testProjectMembersPage.totalCount = 1;
        testProjectMembersPage.pageCount = 1;

        store.commit(PROJECT_MEMBER_MUTATIONS.FETCH, testProjectMembersPage);

        const wrapper = mount(ProjectMembersArea, {
            store,
            localVue,
            mocks: {
                $route: {
                    query: {}
                }
            }
        });

        expect(wrapper.vm.projectMembers.length).toBe(1);

        store.commit(PROJECT_MEMBER_MUTATIONS.DELETE, [projectMember1.user.email]);

        expect(wrapper.vm.projectMembers.length).toBe(0);
    });
});
