// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { VNode } from 'vue';
import { DirectiveBinding } from 'vue/types/options';
import Vuex from 'vuex';

import EditProjectDropdown from '@/components/navigation/EditProjectDropdown.vue';

import { router } from '@/router';
import { appStateModule } from '@/store/modules/appState';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { Project } from '@/types/projects';
import { createLocalVue, mount } from '@vue/test-utils';

import { ProjectsApiMock } from '../mock/api/projects';
import { makeNotificationsModule } from "@/store/modules/notifications";
import { NotificatorPlugin } from "@/utils/plugins/notificator";

const localVue = createLocalVue();
localVue.use(Vuex);

const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const notificationsModule = makeNotificationsModule();
const store = new Vuex.Store({ modules: { projectsModule, appStateModule, notificationsModule }});
const project = new Project('id', 'test', 'test', 'test', 'ownedId', false);

localVue.use(new NotificatorPlugin(store));

let clickOutsideEvent: EventListener;

localVue.directive('cli' +
    'ck-outside', {
    bind: function (el: HTMLElement, binding: DirectiveBinding, vnode: VNode) {
        clickOutsideEvent = function(event: Event): void {
            if (el === event.target) {
                return;
            }

            if (vnode.context && binding.expression) {
                vnode.context[binding.expression](event);
            }
        };

        document.body.addEventListener('click', clickOutsideEvent);
    },
    unbind: function(): void {
        document.body.removeEventListener('click', clickOutsideEvent);
    },
});


store.commit(PROJECTS_MUTATIONS.ADD, project);
store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, project.id);

describe('EditProjectDropdown', () => {
    it('renders correctly', (): void => {
        const wrapper = mount(EditProjectDropdown, {
            store,
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('dropdown opens correctly', async (): Promise<void> => {
        const wrapper = mount(EditProjectDropdown, {
            store,
            localVue,
            router,
        });

        await wrapper.find('.edit-project__selection-area').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });
});
