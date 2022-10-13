// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { BucketsMock } from '../../mock/api/buckets';
import { ProjectsApiMock } from '../../mock/api/projects';

import { BUCKET_MUTATIONS, makeBucketsModule } from '@/store/modules/buckets';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { Bucket, BucketPage } from '@/types/buckets';
import { Project } from '@/types/projects';

import BucketArea from '@/components/project/buckets/BucketArea.vue';

const localVue = createLocalVue();
localVue.use(Vuex);

const bucketsApi = new BucketsMock();
const bucketUsageModule = makeBucketsModule(bucketsApi);
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);

const store = new Vuex.Store({ modules: { bucketUsageModule, projectsModule } });
const bucket = new Bucket('name', 1, 1, 1, 1, new Date(), new Date());

describe('BucketArea.vue', () => {
    it('renders correctly without bucket', (): void => {
        const project = new Project('id', 'test', 'test', 'test', 'ownedId', false);
        store.commit(PROJECTS_MUTATIONS.ADD, project);
        store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, project.id);

        const wrapper = shallowMount(BucketArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with bucket', (): void => {
        const bucketPage = new BucketPage([bucket], '', 5, 0, 1, 1, 1);
        store.commit(BUCKET_MUTATIONS.SET, bucketPage);
        store.commit(BUCKET_MUTATIONS.SET_PAGE, 1);

        const wrapper = shallowMount(BucketArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with pagination', (): void => {
        const newBucketPage = new BucketPage([bucket, bucket, bucket, bucket, bucket, bucket, bucket, bucket], '', 7, 1, 2, 1, 8);
        store.commit(BUCKET_MUTATIONS.SET, newBucketPage);
        store.commit(BUCKET_MUTATIONS.SET_PAGE, 1);

        const wrapper = shallowMount(BucketArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly without search results', (): void => {
        const newBucketPage = new BucketPage([], 'test', 7, 0, 0, 0, 0);
        store.commit(BUCKET_MUTATIONS.SET, newBucketPage);
        store.commit(BUCKET_MUTATIONS.SET_PAGE, 1);
        store.commit(BUCKET_MUTATIONS.SET_SEARCH, 'test');

        const wrapper = shallowMount(BucketArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
