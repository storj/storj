// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <Icon />
                    <span class="modal__header__title">Join project</span>
                </div>
                <hr>
                <div class="modal__info">
                    Join the {{ invite.projectName }} team project.
                </div>
                <hr>
                <div class="modal__buttons">
                    <VButton
                        class="modal__buttons__button"
                        width="calc(50% - 8px)"
                        border-radius="8px"
                        font-size="14px"
                        :is-transparent="true"
                        :is-disabled="isLoading"
                        :on-press="() => respondToInvitation(ProjectInvitationResponse.Decline)"
                        label="Decline"
                    />
                    <VButton
                        class="modal__buttons__button"
                        width="calc(50% - 8px)"
                        border-radius="8px"
                        font-size="14px"
                        :is-disabled="isLoading"
                        :on-press="() => respondToInvitation(ProjectInvitationResponse.Accept)"
                        label="Join Project"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';

import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/utils/hooks';
import { ProjectInvitation, ProjectInvitationResponse } from '@/types/projects';
import { AnalyticsHttpApi } from '@/api/analytics';
import { LocalData } from '@/utils/localData';
import { RouteConfig } from '@/router';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';

import Icon from '@/../static/images/modals/boxesIcon.svg';

const appStore = useAppStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const router = useRouter();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const isLoading = ref<boolean>(false);

/**
 * Returns selected project member invitation from the store.
 */
const invite = computed((): ProjectInvitation => {
    return projectsStore.state.selectedInvitation;
});

/**
 * Handles accepting or declining the project member invitation.
 */
async function respondToInvitation(response: ProjectInvitationResponse): Promise<void> {
    if (isLoading.value) return;
    isLoading.value = true;

    let success = false;
    try {
        await projectsStore.respondToInvitation(invite.value.projectID, response);
        success = true;
    } catch (error) {
        const action = response === ProjectInvitationResponse.Accept ? 'accept' : 'decline';
        notify.error(`Failed to ${action} project invitation. ${error.message}`, null);
    }

    try {
        await projectsStore.getUserInvitations();
        await projectsStore.getProjects();
    } catch (error) {
        notify.error(`Failed to reload projects and invitations list. ${error.message}`, null);
    }

    if (!success) {
        isLoading.value = false;
        return;
    }

    if (response === ProjectInvitationResponse.Accept) {
        projectsStore.selectProject(invite.value.projectID);
        LocalData.setSelectedProjectId(invite.value.projectID);

        analytics.pageVisit(RouteConfig.ProjectDashboard.path);
        router.push(RouteConfig.ProjectDashboard.path);
    }

    closeModal();
}

/**
 * Closes modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}
</script>

<style scoped lang="scss">
.modal {
    width: 410px;
    padding: 32px;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: 16px;

    @media screen and (width <= 460px) {
        width: calc(100vw - 48px);
    }

    &__header {
        display: flex;
        gap: 16px;
        align-items: center;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
        }
    }

    &__info {
        font-family: 'font_regular', sans-serif;
        font-size: 14px;
        line-height: 20px;
        text-align: left;
    }

    &__buttons {
        display: flex;
        gap: 16px;
        justify-content: space-between;

        &__button {
            padding: 12px 0;
            line-height: 24px;
        }
    }

    & > hr {
        height: 1px;
        border: none;
        background-color: var(--c-grey-2);
    }
}
</style>
