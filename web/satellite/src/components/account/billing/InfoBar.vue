// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="billing-info-bar">
        <div class="billing-info-bar__info">
            <VLoader class="billing-info-loader" v-if="isDataFetching" width="20px" height="20px"/>
            <b v-else class="billing-info-bar__info__storage-value">{{ storageRemaining }}</b>
            <span class="billing-info-bar__info__storage-label">of Storage Remaining</span>
            <VLoader class="billing-info-loader" v-if="isDataFetching" width="20px" height="20px"/>
            <b v-else class="billing-info-bar__info__bandwidth-value">{{ bandwidthRemaining }}</b>
            <span class="billing-info-bar__info__bandwidth-label">of Bandwidth Remaining</span>
            <router-link class="billing-info-bar__info__button" :to="projectDashboardPath">Details</router-link>
        </div>
        <a
            class="billing-info-bar__link"
            :href="projectLimitsIncreaseRequestURL"
            target="_blank"
            rel="noopener noreferrer"
        >
            Request Limit Increase ->
        </a>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VLoader from '@/components/common/VLoader.vue';

import { RouteConfig } from '@/router';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { Size } from '@/utils/bytesSize';
import { MetaUtils } from '@/utils/meta';

/**
 * VBanner is common banner for needed pages
 */
@Component({
    components: {
        VLoader,
    },
})
export default class InfoBar extends Vue {
    public readonly projectDashboardPath: string = RouteConfig.ProjectDashboard.path;
    public isDataFetching: boolean = true;

    /**
     * Lifecycle hook after initial render.
     * Fetches project limits.
     */
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);

            this.isDataFetching = false;
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Returns formatted string of remaining storage.
     */
    public get storageRemaining(): string {
        const storageUsed = this.$store.state.projectsModule.currentLimits.storageUsed;
        const storageLimit = this.$store.state.projectsModule.currentLimits.storageLimit;

        const difference = storageLimit - storageUsed;
        if (difference < 0) {
            return '0 Bytes';
        }

        const remaining = new Size(difference, 2);

        return `${remaining.formattedBytes}${remaining.label}`;
    }

    /**
     * Returns formatted string of remaining bandwidth.
     */
    public get bandwidthRemaining(): string {
        const bandwidthUsed = this.$store.state.projectsModule.currentLimits.bandwidthUsed;
        const bandwidthLimit = this.$store.state.projectsModule.currentLimits.bandwidthLimit;

        const difference = bandwidthLimit - bandwidthUsed;
        if (difference < 0) {
            return '0 Bytes';
        }

        const remaining = new Size(difference, 2);

        return `${remaining.formattedBytes}${remaining.label}`;
    }

    /**
     * Returns project limits increase request url from config.
     */
    public get projectLimitsIncreaseRequestURL(): string {
        return MetaUtils.getMetaContent('project-limits-increase-request-url');
    }
}
</script>

<style scoped lang="scss">
    .billing-info-bar {
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        display: flex;
        justify-content: space-between;
        align-items: center;
        background-color: #dce5f2;
        width: calc(100% - 60px);
        padding: 10px 30px;
        font-family: 'font_regular', sans-serif;
        color: #2b3543;

        &__info {
            display: flex;
            align-items: center;

            &__storage-value,
            &__storage-label,
            &__bandwidth-value,
            &__bandwidth-label {
                margin-right: 5px;
                font-size: 14px;
                line-height: 17px;
                white-space: nowrap;
            }

            &__button {
                padding: 5px 10px;
                margin-left: 15px;
                background-color: #fff;
                opacity: 0.8;
                border-radius: 6px;
                font-size: 12px;
                line-height: 15px;
                letter-spacing: 0.02em;
                color: rgba(43, 53, 67, 0.5);
            }
        }

        &__link {
            font-size: 14px;
            line-height: 17px;
            font-family: 'font_medium', sans-serif;
            color: #2683ff;
        }
    }

    .billing-info-loader {
        margin-right: 5px;
    }

    @media screen and (max-width: 768px) {

        .billing-info-bar {

            &__info {

                &__storage-value,
                &__storage-label,
                &__bandwidth-value,
                &__bandwidth-label {
                    white-space: unset;
                }
            }
        }
    }
</style>
