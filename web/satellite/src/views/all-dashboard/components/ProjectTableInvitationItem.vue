// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        item-type="project"
        :item="itemToRender"
        class="invitation-item"
    >
        <template #options>
            <th class="options overflow-visible">
                <v-button
                    :loading="isLoading"
                    :disabled="isLoading"
                    :on-press="onJoinClicked"
                    border-radius="8px"
                    font-size="12px"
                    label="Join Project"
                    class="invitation-item__menu__button"
                />
                <v-button
                    :loading="isLoading"
                    :disabled="isLoading"
                    :on-press="onJoinClicked"
                    border-radius="8px"
                    font-size="12px"
                    label="Join"
                    class="invitation-item__menu__mobile-button"
                />
                <div class="invitation-item__menu">
                    <div class="invitation-item__menu__icon" @click.stop="toggleDropDown">
                        <div class="invitation-item__menu__icon__content" :class="{open: isDropdownOpen}">
                            <menu-icon />
                        </div>
                    </div>

                    <div v-if="isDropdownOpen" v-click-outside="closeDropDown" class="invitation-item__menu__dropdown">
                        <div class="invitation-item__menu__dropdown__item" @click.stop="onDeclineClicked">
                            <logout-icon />
                            <p class="invitation-item__menu__dropdown__item__label">Decline invite</p>
                        </div>
                    </div>
                </div>
            </th>
        </template>
        <menu-icon />
    </table-item>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { ProjectInvitation, ProjectInvitationResponse } from '@/types/projects';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useResize } from '@/composables/resize';
import { AnalyticsHttpApi } from '@/api/analytics';

import VButton from '@/components/common/VButton.vue';
import TableItem from '@/components/common/TableItem.vue';

import MenuIcon from '@/../static/images/common/horizontalDots.svg';
import LogoutIcon from '@/../static/images/navigation/logout.svg';

const appStore = useAppStore();
const projectsStore = useProjectsStore();
const notify = useNotify();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const isLoading = ref<boolean>(false);

const props = defineProps<{
    invitation: ProjectInvitation,
}>();

const { isMobile } = useResize();

const itemToRender = computed((): { [key: string]: unknown | string[] } => {
    if (!isMobile.value) {
        return {
            multi: { title: props.invitation.projectName, subtitle: props.invitation.projectDescription },
            date: props.invitation.invitedDate,
            memberCount: '',
            invited: true,
        };
    }

    return { info: [ props.invitation.projectName, props.invitation.projectDescription ] };
});

/**
 * isDropdownOpen if dropdown is open.
 */
const isDropdownOpen = computed((): boolean => {
    return appStore.state.activeDropdown === props.invitation.projectID;
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
        analytics.eventTriggered(AnalyticsEvent.PROJECT_INVITATION_DECLINED);
    } catch (error) {
        notify.error(`Failed to decline project invitation. ${error.message}`, AnalyticsErrorEventSource.PROJECT_INVITATION);
    }

    try {
        await projectsStore.getUserInvitations();
        await projectsStore.getProjects();
    } catch (error) {
        notify.error(`Failed to reload projects and invitations list. ${error.message}`, AnalyticsErrorEventSource.PROJECT_INVITATION);
    }

    isLoading.value = false;
}

function toggleDropDown() {
    appStore.toggleActiveDropdown(props.invitation.projectID);
}

function closeDropDown() {
    appStore.closeDropdowns();
}

</script>

<style scoped lang="scss">
.invitation-item {

    .options {
        display: flex;
        align-items: center;
        justify-content: flex-end;
        column-gap: 10px;
        padding-right: 10px;
    }

    &__menu {
        position: relative;
        cursor: pointer;

        &__button {
            padding: 10px 16px;

            @media screen and (width <= 500px) {
                display: none;
            }
        }

        &__mobile-button {
            display: none;
            padding: 10px 16px;

            @media screen and (width <= 500px) {
                display: flex;
            }
        }

        &__icon {

            &__content {
                height: 32px;
                width: 32px;
                padding: 12px 5px;
                border-radius: 5px;
                box-sizing: border-box;
                display: flex;
                align-items: center;
                justify-content: center;

                &.open {
                    background: var(--c-grey-3);
                }
            }
        }

        &__dropdown {
            position: absolute;
            top: 40px;
            right: 0;
            background: #fff;
            box-shadow: 0 7px 20px rgb(0 0 0 / 15%);
            border: 1px solid var(--c-grey-2);
            border-radius: 8px;
            z-index: 100;
            overflow: hidden;

            &__item {
                display: flex;
                align-items: center;
                width: 200px;
                padding: 15px;
                color: var(--c-grey-6);
                cursor: pointer;

                &__label {
                    font-family: 'font_regular', sans-serif;
                    margin: 0 0 0 10px;
                }

                &:hover {
                    font-family: 'font_medium', sans-serif;
                    color: var(--c-blue-3);
                    background-color: var(--c-grey-1);

                    svg :deep(path) {
                        fill: var(--c-blue-3);
                    }
                }
            }
        }
    }
}
</style>
