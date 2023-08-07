// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-project">
        <div class="create-project__container">
            <div class="create-project__container__image-container">
                <img
                    class="create-project__container__image-container__img"
                    src="@/../static/images/project/createProject.png"
                    alt="create project"
                >
            </div>
            <h2 class="create-project__container__title" aria-roledescription="title">Create a Project</h2>
            <VInput
                label="Project Name"
                additional-label="Up To 20 Characters"
                placeholder="Enter Project Name"
                class="full-input"
                is-limit-shown
                :current-limit="projectName.length"
                :max-symbols="20"
                :error="nameError"
                @setData="setProjectName"
            />
            <VInput
                label="Description"
                placeholder="Enter Project Description"
                additional-label="Optional"
                class="full-input"
                is-multiline
                height="100px"
                is-limit-shown
                :current-limit="description.length"
                :max-symbols="100"
                @setData="setProjectDescription"
            />
            <div class="create-project__container__button-container">
                <VButton
                    class="create-project__container__button-container__cancel"
                    label="Cancel"
                    width="210px"
                    height="48px"
                    :on-press="onCancelClick"
                    :is-transparent="true"
                />
                <VButton
                    label="Create Project +"
                    width="210px"
                    height="48px"
                    :on-press="onCreateProjectClick"
                    :is-disabled="!projectName"
                />
            </div>
            <div v-if="isLoading" class="create-project__container__blur">
                <VLoader class="create-project__container__blur__loader" width="50px" height="50px" />
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { ProjectFields } from '@/types/projects';
import { LocalData } from '@/utils/localData';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VLoader from '@/components/common/VLoader.vue';
import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';

const analyticsStore = useAnalyticsStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const router = useRouter();

const description = ref<string>('');
const createdProjectId = ref<string>('');
const projectName = ref<string>('');
const nameError = ref<string>('');
const isLoading = ref<boolean>(false);

/**
 * Sets project name from input value.
 */
function setProjectName(value: string): void {
    projectName.value = value;
    nameError.value = '';
}

/**
 * Sets project description from input value.
 */
function setProjectDescription(value: string): void {
    description.value = value;
}

/**
 * Redirects to previous route.
 */
function onCancelClick(): void {
    const PREVIOUS_ROUTE_NUMBER = -1;
    router.go(PREVIOUS_ROUTE_NUMBER);
}

/**
 * Creates project and refreshes store.
 */
async function onCreateProjectClick(): Promise<void> {
    if (isLoading.value) return;

    isLoading.value = true;
    projectName.value = projectName.value.trim();

    const project = new ProjectFields(
        projectName.value,
        description.value,
        usersStore.state.user.id,
    );

    try {
        project.checkName();
    } catch (error) {
        isLoading.value = false;
        nameError.value = error.message;
        analyticsStore.errorEventTriggered(AnalyticsErrorEventSource.CREATE_PROJECT_MODAL);

        return;
    }

    try {
        createdProjectId.value = await projectsStore.createProject(project);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.CREATE_PROJECT_MODAL);
        isLoading.value = false;

        return;
    }

    selectCreatedProject();

    notify.success('Project created successfully!');

    isLoading.value = false;

    analyticsStore.pageVisit(RouteConfig.ProjectDashboard.path);
    await router.push(RouteConfig.ProjectDashboard.path);
}

/**
 * Selects just created project.
 */
function selectCreatedProject(): void {
    projectsStore.selectProject(createdProjectId.value);
    LocalData.setSelectedProjectId(createdProjectId.value);
}
</script>

<style scoped lang="scss">
    .create-project {
        width: 100%;
        height: calc(100% - 140px);
        padding: 70px 0;
        font-family: 'font_regular', sans-serif;

        &__container {
            margin: 0 auto;
            max-width: 440px;
            padding: 70px 50px 55px;
            background-color: #fff;
            border-radius: 8px;
            position: relative;

            &__image-container {
                width: 100%;
                display: flex;
                justify-content: center;
            }

            &__img {
                max-width: 190px;
                max-height: 130px;
            }

            &__title {
                font-size: 28px;
                line-height: 34px;
                color: #384b65;
                font-family: 'font_bold', sans-serif;
                text-align: center;
                margin: 15px 0 30px;
            }

            &__button-container {
                width: 100%;
                display: flex;
                align-items: center;
                justify-content: space-between;
                margin-top: 30px;

                &__cancel {
                    margin-right: 20px;
                }
            }

            &__blur {
                position: absolute;
                top: 0;
                left: 0;
                height: 100%;
                width: 100%;
                background-color: rgb(229 229 229 / 20%);
                border-radius: 8px;
                z-index: 100;

                &__loader {
                    width: 25px;
                    height: 25px;
                    position: absolute;
                    right: 40px;
                    top: 40px;
                }
            }
        }
    }

    .full-input {
        margin-top: 20px;
    }
</style>
