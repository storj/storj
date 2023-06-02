// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="invite" class="banner">
        <VLoader v-if="isLoading" class="banner__loader" width="40px" height="40px" />
        <span v-if="invites.length > 1" class="banner__count">{{ invites.length }}</span>
        <div class="banner__left">
            <UsersIcon class="banner__left__icon" />
            <span>{{ invite.inviterEmail }} has invited you to the project "{{ invite.projectName }}".</span>
        </div>
        <div class="banner__right">
            <div class="banner__right__links">
                <a @click="onJoinClicked">Join Project</a>
                <a @click="onDeclineClicked">Decline</a>
            </div>
            <CloseIcon class="banner__right__close" @click="onCloseClicked" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { ProjectInvitation, ProjectInvitationResponse } from '@/types/projects';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/utils/hooks';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import VLoader from '@/components/common/VLoader.vue';

import UsersIcon from '@/../static/images/notifications/usersIcon.svg';
import CloseIcon from '@/../static/images/notifications/closeSmall.svg';

const appStore = useAppStore();
const projectsStore = useProjectsStore();
const notify = useNotify();

const isLoading = ref<boolean>(false);
const hidden = ref<Set<ProjectInvitation>>(new Set<ProjectInvitation>());

/**
 * Returns a sorted list of non-hidden project member invitation from the store.
 */
const invites = computed((): ProjectInvitation[] => {
    return projectsStore.state.invitations
        .filter(invite => !hidden.value.has(invite))
        .sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime());
});

/**
 * Returns the first non-hidden project member invitation from the store.
 */
const invite = computed((): ProjectInvitation | null => {
    return !invites.value.length ? null : invites.value[0];
});

/**
 * Hides the active project member invitation.
 * Closes the notification if there are no more invitations.
 */
function onCloseClicked(): void {
    if (isLoading.value || !invite.value) return;
    hidden.value.add(invite.value);
}

/**
 * Displays the Join Project modal.
 */
function onJoinClicked(): void {
    if (isLoading.value || !invite.value) return;
    projectsStore.selectInvitation(invite.value);
    appStore.updateActiveModal(MODALS.joinProject);
}

/**
 * Declines the project member invitation.
 */
async function onDeclineClicked(): Promise<void> {
    if (isLoading.value || !invite.value) return;
    isLoading.value = true;

    try {
        await projectsStore.respondToInvitation(invite.value.projectID, ProjectInvitationResponse.Decline);
    } catch (error) {
        notify.error(`Failed to decline project invitation. ${error.message}`, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
    }

    try {
        await projectsStore.getUserInvitations();
        await projectsStore.getProjects();
    } catch (error) {
        notify.error(`Failed to reload projects and invitations list. ${error.message}`, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
    }

    isLoading.value = false;
}

</script>

<style scoped lang="scss">
.banner {
    position: relative;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    padding: 16px;
    background: var(--c-white);
    border: 1px solid var(--c-grey-3);
    border-radius: 10px;
    box-shadow: 0 7px 20px rgb(0 0 0 / 15%);
    font-family: 'font_regular', sans-serif;
    font-size: 14px;
    line-height: 20px;

    &__count {
        padding: 2px 8px;
        position: absolute;
        top: -8px;
        left: -8px;
        border-radius: 8px;
        background-color: var(--c-blue-3);
        color: var(--c-white);
    }

    &__loader {
        position: absolute;
        inset: 0;
        align-items: center;
        border-radius: 10px;
        background-color: rgb(255 255 255 / 66%);
    }

    &__left {
        display: flex;
        align-items: center;
        gap: 16px;

        &__icon {
            flex-shrink: 0;
        }
    }

    &__right {
        display: flex;
        align-items: center;
        gap: 16px;

        &__links {
            display: flex;
            align-items: center;
            gap: 23px;
            text-align: center;

            & a {
                color: var(--c-black);
                line-height: 22px;
                text-decoration: underline !important;
            }
        }

        &__close {
            flex-shrink: 0;
            cursor: pointer;
        }
    }
}
</style>
