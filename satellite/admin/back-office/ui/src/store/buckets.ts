// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import {
    BucketInfoPage,
    ProjectManagementHttpApiV1,
} from '@/api/client.gen';
import { BucketCursor } from '@/types/bucket';

class BucketsState {}

export const useBucketsStore = defineStore('buckets', () => {
    const state = reactive<BucketsState>(new BucketsState());

    const projectApi = new ProjectManagementHttpApiV1();

    async function getBuckets(projectID: string, cursor: BucketCursor): Promise<BucketInfoPage> {
        const now = new Date();
        const since = new Date(Date.UTC(
            now.getUTCFullYear(),
            now.getUTCMonth(),
            1,
        ));
        return await projectApi.getProjectBuckets(projectID, cursor.search || '-', cursor.page.toString(), cursor.limit.toString(),
            since.toISOString(), now.toISOString(),
        );
    }

    return {
        state,
        getBuckets,
    };
});
