// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="projects-info-bar">
        <div class="projects-info-bar__info">
            <p class="projects-info-bar__info__message">
                You have used
                <VLoader v-if="isDataFetching" class="pr-info-loader" is-white="true" width="15px" height="15px"/>
                <span class="projects-info-bar__info__message__value" v-else>{{ projectsCount }}</span>
                of your
                <VLoader v-if="isDataFetching" class="pr-info-loader" is-white="true" width="15px" height="15px"/>
                <span class="projects-info-bar__info__message__value" v-else>{{ projectLimit }}</span>
                available projects.
            </p>
        </div>
        <a
            class="projects-info-bar__link"
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

import { PROJECTS_ACTIONS } from '@/store/modules/projects';
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
    public isDataFetching: boolean = true;

    /**
     * Lifecycle hook after initial render.
     * Fetches projects.
     */
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.FETCH);

            this.isDataFetching = false;
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Returns user's projects count.
     */
    public get projectsCount(): number {
        return this.$store.getters.projectsCount;
    }

    /**
     * Returns project limit from store.
     */
    public get projectLimit(): number {
        const projectLimit: number = this.$store.getters.user.projectLimit;
        if (projectLimit < this.projectsCount) return this.projectsCount;

        return projectLimit;
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
    .projects-info-bar {
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        display: flex;
        justify-content: space-between;
        align-items: center;
        background-color: #2582ff;
        width: calc(100% - 60px);
        padding: 10px 30px;
        font-family: 'font_regular', sans-serif;
        color: #fff;

        &__info {
            display: flex;
            align-items: center;

            &__message {
                display: flex;
                align-items: center;
                margin-right: 5px;
                font-size: 14px;
                line-height: 17px;
                white-space: nowrap;

                &__value {
                    margin: 0 5px;
                }
            }
        }

        &__link {
            font-size: 14px;
            line-height: 17px;
            font-family: 'font_medium', sans-serif;
            color: #fff;
        }
    }

    .pr-info-loader {
        margin: 0 5px;
    }
</style>
