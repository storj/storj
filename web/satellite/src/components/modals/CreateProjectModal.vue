// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <img
                    class="modal__icon"
                    src="@/../static/images/account/billing/paidTier/prompt.png"
                    alt="Prompt Image"
                >
                <h1 class="modal__title" aria-roledescription="modal-title">
                    Create a Project
                </h1>
                <VInput
                    label="Project Name*"
                    additional-label="Up To 20 Characters"
                    placeholder="Project Name"
                    class="full-input"
                    is-limit-shown
                    :current-limit="projectName.length"
                    :max-symbols="20"
                    :error="nameError"
                    @setData="setProjectName"
                />
                <VInput
                    label="Description - Optional"
                    placeholder="Project Description"
                    class="full-input"
                    is-multiline
                    height="100px"
                    is-limit-shown
                    :current-limit="description.length"
                    :max-symbols="100"
                    @setData="setProjectDescription"
                />
                <div class="modal__button-container">
                    <VButton
                        label="Cancel"
                        width="100%"
                        height="48px"
                        :on-press="closeModal"
                        :is-transparent="true"
                    />
                    <VButton
                        label="Create Project"
                        width="100%"
                        height="48px"
                        :on-press="onCreateProjectClick"
                        :is-disabled="!projectName"
                    />
                </div>
                <div v-if="isLoading" class="modal__blur">
                    <VLoader class="modal__blur__loader" width="50px" height="50px" />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { ProjectFields } from '@/types/projects';
import { LocalData } from '@/utils/localData';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VLoader from '@/components/common/VLoader.vue';
import VInput from '@/components/common/VInput.vue';
import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';

const analyticsStore = useAnalyticsStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
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
    closeModal();

    bucketsStore.clearS3Data();
    if (usersStore.state.settings.passphrasePrompt) {
        appStore.updateActiveModal(MODALS.createProjectPassphrase);
    }

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

/**
 * Closes create project modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}
</script>

<style scoped lang="scss">
    .modal {
        width: 400px;
        padding: 54px 48px 51px;
        display: flex;
        align-items: center;
        flex-direction: column;
        font-family: 'font_regular', sans-serif;

        @media screen and (width <= 550px) {
            width: calc(100% - 48px);
            padding: 54px 24px 32px;
        }

        &__icon {
            max-height: 154px;
            max-width: 118px;

            @media screen and (width <= 550px) {
                max-height: 77px;
                max-width: 59px;
            }
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 34px;
            color: #1b2533;
            margin-top: 40px;
            text-align: center;

            @media screen and (width <= 550px) {
                margin-top: 16px;
                font-size: 24px;
                line-height: 31px;
            }
        }

        &__info {
            font-family: 'font_regular', sans-serif;
            font-size: 16px;
            line-height: 21px;
            text-align: center;
            color: #354049;
            margin: 15px 0 45px;
        }

        &__button-container {
            width: 100%;
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-top: 30px;
            column-gap: 20px;

            @media screen and (width <= 550px) {
                margin-top: 20px;
                column-gap: unset;
                row-gap: 8px;
                flex-direction: column-reverse;
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

    .full-input {
        margin-top: 20px;
    }

    @media screen and (width <= 550px) {

        :deep(.add-label) {
            display: none;
        }
    }
</style>
