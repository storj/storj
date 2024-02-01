// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-breadcrumbs :items="items" active-class="font-weight-bold" class="pa-0">
        <template #divider>
            <img src="@/assets/icon-right.svg" alt="Breadcrumbs separator" width="10">
        </template>
    </v-breadcrumbs>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VBreadcrumbs } from 'vuetify/components';

import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { ROUTES } from '@/router';

const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();

/**
 * Returns the name of the selected bucket.
 */
const bucketName = computed<string>(() => bucketsStore.state.fileComponentBucketName);

/**
 * Returns the name of the current path within the selected bucket.
 */
const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

type BreadcrumbItem = {
    title: string;
    to: string;
}

/**
 * Returns breadcrumb items corresponding to parts in the file browser path.
 */
const items = computed<BreadcrumbItem[]>(() => {
    const bucketsURL = `${ROUTES.Projects.path}/${projectsStore.state.selectedProject.urlId}/${ROUTES.Buckets.path}`;

    const pathParts = [bucketName.value];
    if (filePath.value) pathParts.push(...filePath.value.split('/'));

    return [
        { title: 'Buckets', to: bucketsURL },
        ...pathParts.map<BreadcrumbItem>((part, index) => {
            const suffix = pathParts.slice(0, index + 1)
                .map(part => encodeURIComponent(part))
                .join('/');
            return { title: part, to: `${bucketsURL}/${suffix}` };
        }),
    ];
});
</script>
