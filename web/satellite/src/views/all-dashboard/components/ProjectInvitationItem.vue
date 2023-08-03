// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="invite-item">
        <project-ownership-tag class="invite-item__tag" :role="ProjectRole.Invited" />

        <div class="invite-item__info">
            <p class="invite-item__info__name">{{ invitation.projectName }}</p>
            <p class="invite-item__info__description">{{ invitation.projectDescription }}</p>
        </div>

        <div class="invite-item__buttons">
            <VButton
                class="invite-item__buttons__button"
                width="fit-content"
                border-radius="8px"
                font-size="12px"
                :is-disabled="isLoading"
                :on-press="onJoinClicked"
            >
                <span class="invite-item__buttons__button__join-label">Join Project</span>
                <span class="invite-item__buttons__button__join-label-mobile">Join</span>
            </VButton>
            <VButton
                class="invite-item__buttons__button decline"
                border-radius="8px"
                font-size="12px"
                :is-disabled="isLoading"
                :is-transparent="true"
                :on-press="onDeclineClicked"
                label="Decline"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { ProjectRole } from '@/types/projectMembers';
import { ProjectInvitation, ProjectInvitationResponse } from '@/types/projects';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VButton from '@/components/common/VButton.vue';
import ProjectOwnershipTag from '@/components/project/ProjectOwnershipTag.vue';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();
const notify = useNotify();

const isLoading = ref<boolean>(false);

const props = withDefaults(defineProps<{
    invitation?: ProjectInvitation,
}>(), {
    invitation: () => new ProjectInvitation('', '', '', '', new Date()),
});

/**
 * Displays the Join Project modal.
 */
function onJoinClicked(): void {
    projectsStore.selectInvitation(props.invitation);
    appStore.updateActiveModal(MODALS.joinProject);
}

/**
 * Declines the project member invitation.
 */
async function onDeclineClicked(): Promise<void> {
    if (isLoading.value) return;
    isLoading.value = true;

    try {
        await projectsStore.respondToInvitation(props.invitation.projectID, ProjectInvitationResponse.Decline);
        analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_INVITATION_DECLINED);
    } catch (error) {
        error.message = `Failed to decline project invitation. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_INVITATION);
    }

    try {
        await projectsStore.getUserInvitations();
        await projectsStore.getProjects();
    } catch (error) {
        error.message = `Failed to reload projects and invitations list. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_INVITATION);
    }

    isLoading.value = false;
}

</script>

<style scoped lang="scss">
.invite-item {
    display: flex;
    align-items: stretch;
    flex-direction: column;
    gap: 16px;
    padding: 24px;
    background: var(--c-white);
    box-shadow: 0 0 20px rgb(0 0 0 / 5%);
    border-radius: 8px;

    &__tag {
        align-self: flex-start;
    }

    &__info {
        display: flex;
        gap: 4px;
        flex-direction: column;

        &__name {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
            white-space: nowrap;
            text-overflow: ellipsis;
            overflow: hidden;
            text-align: start;
        }

        &__description {
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            min-height: 20px;
            color: var(--c-grey-6);
            line-height: 20px;
            white-space: nowrap;
            text-overflow: ellipsis;
            overflow: hidden;
        }
    }

    &__buttons {
        display: flex;
        gap: 10px;

        &__button {
            line-height: 20px;
            padding: 10px 16px;

            &__join-label-mobile {
                display: none;
            }

            @media screen and (width <= 520px) and (width > 425px) {

                &__join-label {
                    display: none;
                }

                &__join-label-mobile {
                    display: inline;
                }
            }
        }
    }
}
</style>
