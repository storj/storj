// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="mb-3">
        <div class="d-inline">
            <a class="d-inline path-buckets" @click="() => $emit('bucketClick')">Buckets</a>
            <svg
                class="mx-3"
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
        </div>

        <div v-for="(path, idx) in crumbs" :key="idx" class="d-inline">
            <router-link :to="link(idx)">
                <a class="path" href="javascript:null">{{ path }}</a>
            </router-link>

            <svg
                v-if="displayDivider(idx)"
                class="mx-3"
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
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class BreadCrumbs extends Vue {
    /**
     * Retrieves the current bucket name from the store.
     */
    private get bucketName(): string {
        return this.$store.state.files.bucket;
    }

    /**
     * Retrieves the current path from the store and creates an array of folders for the bread crumbs that the user can click on.
     */
    public get crumbs(): string[] {
        let path: string[] = this.$store.state.files.path.split('/');
        path =
            path.length > 1
                ? [this.bucketName, ...path.slice(0, path.length - 1)]
                : [this.bucketName];
        return path;
    }

    /**
     * Returns a link to the folder at the current breadcrumb index.
     */
    public link(idx: number): string {
        const crumbs = this.crumbs;
        let path = '';
        if (idx > 0) path = crumbs.slice(1, idx + 1).join('/') + '/';
        return this.$store.state.files.browserRoot + path;
    }

    /**
     * Returns a boolean denoting if a divider needs to be displayed at current breadcrumb index.
     */
    public displayDivider(idx: number): boolean {
        const length = this.crumbs.length;
        return (idx !== 0 || length > 1) && idx !== length - 1;
    }
}
</script>

<style scoped lang="css">
.path {
    font-family: 'font_bold', sans-serif;
    font-size: 14px;
    color: #1b2533;
    font-weight: bold;
    cursor: pointer;
}

.path:hover {
    color: #376fff;
}

.path-buckets {
    font-size: 14px;
    color: #232a34;
    cursor: pointer;
}

.path-buckets:hover {
    color: #376fff;
}
</style>
