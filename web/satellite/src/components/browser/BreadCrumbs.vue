// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <p class="breadcrumbs">
        <span class="breadcrumbs__item">
            <a class="breadcrumbs__item__text" @click="bucketClick">Buckets</a>
            <svg
                class="breadcrumbs__item__chevron"
                width="6"
                height="11"
                viewBox="0 0 6 11"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
            >
                <path
                    opacity="0.5"
                    fill-rule="evenodd"
                    clip-rule="evenodd"
                    d="M0.254166 0.280039C-0.0847221 0.653424 -0.0847221 1.2588 0.254166 1.63219L3.54555 5.25862L0.254166 8.88505C-0.0847225 9.25844 -0.0847225 9.86382 0.254166 10.2372C0.593054 10.6106 1.1425 10.6106 1.48139 10.2372L6 5.25862L1.48139 0.280039C1.1425 -0.0933463 0.593054 -0.0933463 0.254166 0.280039Z"
                    fill="black"
                />
            </svg>
        </span>

        <span v-for="(path, idx) in crumbs" :key="idx" class="breadcrumbs__item">
            <a class="breadcrumbs__item__text path" @click.prevent="redirectToCrumb(idx)">{{ path }}</a>

            <svg
                v-if="displayDivider(idx)"
                class="breadcrumbs__item__chevron"
                width="6"
                height="11"
                viewBox="0 0 6 11"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
            >
                <path
                    opacity="0.5"
                    fill-rule="evenodd"
                    clip-rule="evenodd"
                    d="M0.254166 0.280039C-0.0847221 0.653424 -0.0847221 1.2588 0.254166 1.63219L3.54555 5.25862L0.254166 8.88505C-0.0847225 9.25844 -0.0847225 9.86382 0.254166 10.2372C0.593054 10.6106 1.1425 10.6106 1.48139 10.2372L6 5.25862L1.48139 0.280039C1.1425 -0.0933463 0.593054 -0.0933463 0.254166 0.280039Z"
                    fill="black"
                />
            </svg>
        </span>
    </p>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';

import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';

const obStore = useObjectBrowserStore();
const router = useRouter();

const emit = defineEmits(['onUpdate', 'bucketClick']);

/**
 * Retrieves the current bucket name from the store.
 */
const bucketName = computed((): string => {
    return obStore.state.bucket;
});

/**
 * Retrieves the current path from the store and creates an array of folders for the bread crumbs that the user can click on.
 */
const crumbs = computed((): string[] => {
    let path: string[] = obStore.state.path.split('/');
    path =
        path.length > 1
            ? [bucketName.value, ...path.slice(0, path.length - 1)]
            : [bucketName.value];
    return path;
});

function bucketClick() {
    emit('bucketClick');
}

/**
 * Redirects to partial upload to bucket buckets path.
 */
async function redirectToCrumb(idx: number): Promise<void> {
    await router.push(link(idx)).catch(_ => {});
    emit('onUpdate');
}

/**
 * Returns a link to the folder at the current breadcrumb index.
 */
function link(idx: number): string {
    let path = '';

    if (idx > 0) path = crumbs.value.slice(1, idx + 1).join('/') + '/';

    return obStore.state.browserRoot + path;
}

/**
 * Returns a boolean denoting if a divider needs to be displayed at current breadcrumb index.
 */
function displayDivider(idx: number): boolean {
    const length = crumbs.value.length;

    return (idx !== 0 || length > 1) && idx !== length - 1;
}
</script>

<style scoped lang="scss">
.path {
    font-family: 'font_bold', sans-serif;
    color: #1b2533;
    font-weight: bold;
}

.breadcrumbs {

    &__item {
        display: inline-flex;
        gap: 7px;
        align-items: center;

        &__text {
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            color: #232a34;
            cursor: pointer;

            &:hover {
                color: #376fff;
            }
        }

        &__chevron {
            margin-right: 7px;
        }
    }
}
</style>
