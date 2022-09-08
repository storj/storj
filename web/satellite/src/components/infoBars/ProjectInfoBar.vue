// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="projects-info-bar">
        <div class="projects-info-bar__info">
            <p class="projects-info-bar__info__message">
                You have used
                <VLoader v-if="isDataFetching" class="pr-info-loader" is-white="true" width="15px" height="15px" />
                <span v-else class="projects-info-bar__info__message__value">{{ projectsCount }}</span>
                of your
                <VLoader v-if="isDataFetching" class="pr-info-loader" is-white="true" width="15px" height="15px" />
                <span v-else class="projects-info-bar__info__message__value">{{ projectLimit }}</span>
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

import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { MetaUtils } from '@/utils/meta';

import VLoader from '@/components/common/VLoader.vue';

/**
 * VBanner is common banner for needed pages
 */
// @vue/component
@Component({
    components: {
        VLoader,
    },
})
export default class ProjectInfoBar extends Vue {
    public isDataFetching = true;

    /**
     * Lifecycle hook after initial render.
     * Fetches projects.
     */
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.FETCH);

            this.isDataFetching = false;
        } catch (error) {
            return;
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
        display: flex;
        justify-content: space-between;
        align-items: center;
        background-color: #2582ff;
        width: 100%;
        box-sizing: border-box;
        padding: 5px 30px;
        font-family: 'font_regular', sans-serif;
        color: #fff;

        &__info {
            display: flex;
            align-items: center;
            justify-content: flex-start;

            &__message {
                display: flex;
                align-items: center;
                margin-right: 5px;
                font-size: 14px;
                line-height: 17px;

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
            text-align: right;
        }
    }

    .pr-info-loader {
        margin: 0 5px;
    }
</style>
