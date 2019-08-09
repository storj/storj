// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, mount, shallowMount } from '@vue/test-utils';
import * as sinon from 'sinon';
import Vuex from 'vuex';
import HeaderArea from '@/components/team/headerArea/HeaderArea.vue';
import { TeamMember } from '@/types/teamMembers';

const localVue = createLocalVue();

localVue.use(Vuex);

describe('appState Actions', () => {
    let store;
    let actions;

    beforeEach(() => {
        actions = {
            toggleAddTeamMembersPopup: jest.fn()
        };

        store = new Vuex.Store({
            modules: {
                appStateModule: {
                    actions
                }
            }
        });
    });

    it('action on onAddUsersClick works correctly', () => {
        const wrapper = mount(HeaderArea, { store, localVue });

        wrapper.vm.onAddUsersClick();

        expect(actions.toggleAddTeamMembersPopup.mock.calls).toHaveLength(1);
    });
});

describe('projectMembers/notification Actions', () => {
    let store;
    let actions;
    let getters;
    let state;
    let teamMember = new TeamMember('test', 'test', 'test@test.test', 'test');
    let teamMember1 = new TeamMember('test1', 'test1', 'test1@test.test', 'test1');
    let searchQuery = 'test';

    beforeEach(() => {
        getters = {
            selectedProjectMembers: () => [teamMember, teamMember1]
        };

        state = {
            searchParameters: {
                searchQuery: ''
            },
        };

        actions = {
            clearProjectMemberSelection: jest.fn(),
            deleteProjectMembers: async () => {
                return {
                    errorMessage: '',
                    isSuccess: true,
                    data: null
                };
            },
            fetchProjectMembers: async () => {
                return {
                    errorMessage: '',
                    isSuccess: true,
                    data: [teamMember, teamMember1]
                };
            },
            setProjectMembersSearchQuery: () => {
                state.searchParameters.searchQuery = searchQuery;
            },
            success: jest.fn()
        };

        store = new Vuex.Store({
            modules: {
                module: {
                    actions,
                    getters,
                    state
                },
            }
        });
    });

    it('action on onClearSelection works correctly', () => {
        let clearSearchSpy = sinon.spy();

        const wrapper = mount(HeaderArea, { store, localVue });

        wrapper.vm.$refs.headerComponent.clearSearch = clearSearchSpy;
        wrapper.vm.onClearSelection();

        expect(actions.clearProjectMemberSelection.mock.calls).toHaveLength(1);
        expect(wrapper.vm.$data.isDeleteClicked).toBe(false);
        expect(clearSearchSpy.callCount).toBe(1);
    });

    it('actions on onDelete works correctly', async () => {
        let clearSearchSpy = sinon.spy();

        const wrapper = mount(HeaderArea, {
            store,
            localVue
        });

        wrapper.vm.$data.headerState = 1;
        wrapper.vm.$data.isDeleteClicked = true;
        wrapper.vm.$refs.headerComponent.clearSearch = clearSearchSpy;

        await wrapper.vm.onDelete();

        const projectMembersEmails = await store.getters.selectedProjectMembers.map((member) => {
            return member.user.email;
        });

        expect(projectMembersEmails).toHaveLength(2);
        expect(actions.deleteProjectMembers).resolves;
        expect(actions.success.mock.calls).toHaveLength(1);
        expect(wrapper.vm.$data.isDeleteClicked).toBe(false);
        expect(clearSearchSpy.callCount).toBe(1);
    });

    it('action on processSearchQuery works correctly', async () => {
        const wrapper = mount(HeaderArea, {
            store,
            localVue
        });

        await wrapper.vm.processSearchQuery(searchQuery);

        expect(state.searchParameters.searchQuery).toMatch('test');
        expect(actions.fetchProjectMembers).resolves;
    });
});

describe('error conditions', () => {
    let store;
    let actions;
    let getters;
    let searchQuery = 'test';

    beforeEach(() => {
        getters = {
            selectedProjectMembers: () => []
        };

        actions = {
            deleteProjectMembers: async () => {
                return {
                    errorMessage: '',
                    isSuccess: false,
                    data: null
                };
            },
            fetchProjectMembers: async () => {
                return {
                    errorMessage: '',
                    isSuccess: false,
                    data: []
                };
            },
            error: jest.fn()
        };

        store = new Vuex.Store({
            modules: {
                module: {
                    actions,
                    getters
                }
            }
        });
    });

    it('function customUserCount if there is only 1 selected user', () => {
        const wrapper = mount(HeaderArea, { store, localVue });

        wrapper.vm.$data.selectedProjectMembers = 1;

        expect(wrapper.vm.userCountTitle).toMatch('user');
    });

    it('function onDelete if delete is not successful', async () => {
        const wrapper = mount(HeaderArea, {
            store,
            localVue
        });

        wrapper.vm.$data.headerState = 1;
        wrapper.vm.$data.isDeleteClicked = true;

        await wrapper.vm.onDelete();

        await store.getters.selectedProjectMembers.map((member) => {
            return member.user.email;
        });

        expect(actions.error.mock.calls).toHaveLength(1);
    });

    it('function processSearchQuery if fetch is not successful', async () => {
        const wrapper = mount(HeaderArea, {
            store,
            localVue
        });

        await wrapper.vm.processSearchQuery(searchQuery);

        expect(actions.error.mock.calls).toHaveLength(1);
    });
});

describe('HeaderArea.vue', () => {
    it('renders correctly', () => {
        const wrapper = shallowMount(HeaderArea);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with default props', () => {
        const wrapper = mount(HeaderArea);

        expect(wrapper.vm.$props.headerState).toBe(0);
        expect(wrapper.vm.$props.selectedProjectMembers).toBe(0);
    });

    it('function customUserCount work correctly', () => {
        const wrapper = mount(HeaderArea);

        expect(wrapper.vm.userCountTitle).toMatch('users');
    });

    it('function onFirstDeleteClick work correctly', () => {
        const wrapper = mount(HeaderArea);

        wrapper.vm.onFirstDeleteClick();

        expect(wrapper.vm.$data.isDeleteClicked).toBe(true);
    });

});

