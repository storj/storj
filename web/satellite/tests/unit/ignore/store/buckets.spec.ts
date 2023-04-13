// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BucketsApiGql } from '@/api/buckets';
import { Bucket, BucketPage } from '@/types/buckets';
import { Project } from '@/types/projects';

const bucketsApi = new BucketsApiGql();

const selectedProject = new Project();
selectedProject.id = '1';

// const state = store.state.bucketsModule;
const bucket = new Bucket('test', 10, 10, 1, 1, new Date(), new Date());
const page: BucketPage = { buckets: [bucket], currentPage: 1, pageCount: 1, offset: 0, limit: 7, search: 'test', totalCount: 1 };

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
    });

    it('success fetch buckets', async () => {
        jest.spyOn(bucketsApi, 'get').mockReturnValue(
            Promise.resolve(page),
        );

        // await store.dispatch(FETCH, 1);

        // expect(state.page).toEqual(page);
        // expect(state.cursor.page).toEqual(1);
    });

    it('fetch throws an error when api call fails', async () => {
        jest.spyOn(bucketsApi, 'get').mockImplementation(() => { throw new Error(); });

        try {
            // await store.dispatch(FETCH, 1);
        } catch (error) {
            // expect(state.page).toEqual(page);
        }
    });

    it('success set search buckets', () => {
        // store.dispatch(SET_SEARCH, 'test');

        // expect(state.cursor.search).toMatch('test');
    });

    it('success clear', () => {
        // store.dispatch(CLEAR);

        // expect(state.cursor).toEqual(new BucketCursor('', 7, 1));
        // expect(state.page).toEqual(new BucketPage([], '', 7, 0, 1, 1, 0));
    });
});

describe('getters', () => {
    const page: BucketPage = { buckets: [bucket], currentPage: 1, pageCount: 1, offset: 0, limit: 7, search: 'test', totalCount: 1 };

    it('page of buckets', async () => {
        jest.spyOn(bucketsApi, 'get').mockReturnValue(
            Promise.resolve(page),
        );

        // await store.dispatch(FETCH, 1);

        // const storePage = store.getters.page;

        // expect(storePage).toEqual(page);
    });

    it('cursor of buckets', () => {
        // store.dispatch(CLEAR);

        // const cursor = store.getters.cursor;

        // expect(cursor).toEqual(new BucketCursor('', 7, 1));
    });
});
