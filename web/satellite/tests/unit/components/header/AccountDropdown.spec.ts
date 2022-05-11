// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import AccountDropdown from '@/components/header/accountDropdown/AccountDropdown.vue';

import { router } from '@/router';
import { createLocalVue, mount } from '@vue/test-utils';

import { appStateModule } from '@/store/modules/appState';
import {makeProjectsModule} from "@/store/modules/projects";
import {makeUsersModule} from "@/store/modules/users";
import {makeAccessGrantsModule} from "@/store/modules/accessGrants";
import {makeBucketsModule} from "@/store/modules/buckets";
import {makeObjectsModule,} from "@/store/modules/objects";
import {makeNotificationsModule} from "@/store/modules/notifications";
import {makeProjectMembersModule} from "@/store/modules/projectMembers";
import {makePaymentsModule} from "@/store/modules/payments";
import {makeFilesModule} from "@/store/modules/files";

import {PaymentsMock} from "../../mock/api/payments";
import {UsersApiMock} from "../../mock/api/users";
import {ProjectsApiMock} from "../../mock/api/projects";
import {AccessGrantsMock} from "../../mock/api/accessGrants";
import {ProjectMembersApiMock} from "../../mock/api/projectMembers";
import {BucketsMock} from "../../mock/api/buckets";

const localVue = createLocalVue();
localVue.use(Vuex);

const paymentsApi = new PaymentsMock();
const usersApi = new UsersApiMock();
const projectsApi = new ProjectsApiMock();
const accessGrantsApi = new AccessGrantsMock();
const projectMembersApi = new ProjectMembersApiMock();
const bucketsApi = new BucketsMock();

const store = new Vuex.Store({ modules: {
    appStateModule,

    notificationsModule: makeNotificationsModule(),
    accessGrantsModule: makeAccessGrantsModule(accessGrantsApi),
    projectMembersModule: makeProjectMembersModule(projectMembersApi),
    paymentsModule: makePaymentsModule(paymentsApi),
    usersModule: makeUsersModule(usersApi),
    projectsModule: makeProjectsModule(projectsApi),
    bucketUsageModule: makeBucketsModule(bucketsApi),
    objectsModule: makeObjectsModule(),
    files: makeFilesModule(),
}});

describe('AccountDropdown', () => {
    it('renders correctly', (): void => {
        const wrapper = mount(AccountDropdown, {
            store,
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('router works correctly', async (): Promise<void> => {
        const routerSpy = jest.spyOn(router, "push");
        const wrapper = mount(AccountDropdown, {
            store,
            localVue,
            router,
        });

        await wrapper.find('.account-dropdown__wrap__item-container').trigger('click');

        expect(routerSpy).toHaveBeenCalled();
    });
});
