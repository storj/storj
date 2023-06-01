// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="projects-info-bar">
        <div class="projects-info-bar__info">
            <p class="projects-info-bar__info__message">
                You have used
                <VLoader v-if="isDataFetching" class="pr-info-loader" :is-white="true" width="15px" height="15px" />
                <span v-else class="projects-info-bar__info__message__value">{{ projectsCount }}</span>
                of your
                <VLoader v-if="isDataFetching" class="pr-info-loader" :is-white="true" width="15px" height="15px" />
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

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';

import VLoader from '@/components/common/VLoader.vue';

const configStore = useConfigStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const notify = useNotify();

const isDataFetching = ref<boolean>(true);

/**
 * Returns user's projects count.
 */
const projectsCount = computed((): number => {
    return projectsStore.projectsCount(usersStore.state.user.id);
});

/**
 * Returns project limit from store.
 */
const projectLimit = computed((): number => {
    const projectLimit: number = usersStore.state.user.projectLimit;
    if (projectLimit < projectsCount.value) return projectsCount.value;

    return projectLimit;
});

/**
 * Returns project limits increase request url from config.
 */
const projectLimitsIncreaseRequestURL = computed((): string => {
    return configStore.state.config.projectLimitsIncreaseRequestURL;
});

/**
 * Lifecycle hook after initial render.
 * Fetch projects.
 */
onMounted(async (): Promise<void> => {
    try {
        await projectsStore.getProjects();

        isDataFetching.value = false;
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.PROJECT_INFO_BAR);
        return;
    }
});
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
