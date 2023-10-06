// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :class="{ 'owner': isProjectOwner }"
        :item="itemToRender"
        :selectable="true"
        :select-disabled="isProjectOwner"
        :selected="model.isSelected()"
        :on-click="(_) => emit('memberClick', model)"
        class="project-member-item"
        @selectClicked="(_) => emit('selectClick', model)"
    >
        <template #options>
            <th class="project-member-item__menu options overflow-visible" @click.stop="toggleDropDown">
                <div v-if="!isProjectOwner" class="project-member-item__menu__icon">
                    <div class="project-member-item__menu__icon__content" :class="{open: isDropdownOpen}">
                        <menu-icon />
                    </div>
                </div>

                <div v-if="isDropdownOpen" v-click-outside="closeDropDown" class="project-member-item__menu__dropdown">
                    <div v-if="model.isPending() && !isExpired" class="project-member-item__menu__dropdown__item" @click.stop="copyLinkClicked">
                        <copy-icon />
                        <p class="project-member-item__menu__dropdown__item__label">Copy invite link</p>
                    </div>

                    <div v-if="model.isPending() && isExpired" class="project-member-item__menu__dropdown__item" @click.stop="resendClicked">
                        <upload-icon />
                        <p class="project-member-item__menu__dropdown__item__label">Resend invite</p>
                    </div>

                    <div class="project-member-item__menu__dropdown__item" @click.stop="deleteClicked">
                        <delete-icon />
                        <p class="project-member-item__menu__dropdown__item__label">Remove member</p>
                    </div>
                </div>
            </th>
        </template>
    </table-item>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { ProjectInvitationItemModel, ProjectMember, ProjectMemberItemModel, ProjectRole } from '@/types/projectMembers';
import { useResize } from '@/composables/resize';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import TableItem from '@/components/common/TableItem.vue';

import UploadIcon from '@/../static/images/common/upload.svg';
import DeleteIcon from '@/../static/images/browser/galleryView/delete.svg';
import MenuIcon from '@/../static/images/common/horizontalDots.svg';
import CopyIcon from '@/../static/images/accessGrants/newCreateFlow/copy.svg';

const { isMobile, isTablet } = useResize();
const { withLoading } = useLoading();

const notify = useNotify();
const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const pmStore = useProjectMembersStore();
const projectsStore = useProjectsStore();

const props = withDefaults(defineProps<{
    model: ProjectMemberItemModel;
}>(), {
    model: () => new ProjectMember('', '', '', new Date(), ''),
});

const emit = defineEmits<{
    (e: 'selectClick', model: ProjectMemberItemModel): void
    (e: 'memberClick', model: ProjectMemberItemModel): void
    (e: 'removeClick', model: ProjectMemberItemModel): void
    (e: 'resendClick', model: ProjectMemberItemModel): void
}>();

const inviteLink = ref('');

const isProjectOwner = computed((): boolean => {
    return props.model.getUserID() === projectsStore.state.selectedProject.ownerId;
});

const isExpired = computed((): boolean => {
    if (!props.model.isPending()) {
        return false;
    }
    const invite = props.model as ProjectInvitationItemModel;
    return invite.expired;
});

const itemToRender = computed((): { [key: string]: unknown } => {
    let role: ProjectRole = ProjectRole.Member;
    if (props.model.isPending()) {
        if ((props.model as ProjectInvitationItemModel).expired) {
            role = ProjectRole.InviteExpired;
        } else {
            role = ProjectRole.Invited;
        }
    } else if (isProjectOwner.value) {
        role = ProjectRole.Owner;
    }

    if (!isMobile.value && !isTablet.value) {
        const dateStr = props.model.getJoinDate().toLocaleDateString('en-US', { day:'numeric', month:'short', year:'numeric' });
        return {
            name: props.model.getName(),
            email: props.model.getEmail(),
            role: role,
            date: dateStr,
        };
    }

    if (isTablet.value) {
        return { name: props.model.getName(), email: props.model.getEmail(), role: role };
    }
    // TODO: change after adding actions button to list item
    return { name: props.model.getName(), email: props.model.getEmail() };
});

/**
 * isDropdownOpen if dropdown is open.
 */
const isDropdownOpen = computed((): boolean => {
    return appStore.state.activeDropdown === props.model.getEmail();
});

function copyLinkClicked() {
    analyticsStore.eventTriggered(AnalyticsEvent.COPY_INVITE_LINK_CLICKED);
    closeDropDown();

    if (inviteLink.value) {
        navigator.clipboard.writeText(inviteLink.value);
        return;
    }
    withLoading(async () => {
        try {
            inviteLink.value = await pmStore.getInviteLink(props.model.getEmail(), projectsStore.state.selectedProject.id);
            navigator.clipboard.writeText(inviteLink.value);
            notify.notify('Invite copied!');
        } catch (error) {
            error.message = `Error getting invite link. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_MEMBERS_PAGE);
        }
    });
}

function resendClicked() {
    emit('resendClick', props.model);
    closeDropDown();
}

function deleteClicked() {
    emit('removeClick', props.model);
    closeDropDown();
}

function toggleDropDown() {
    if (isProjectOwner.value) {
        return;
    }
    appStore.toggleActiveDropdown(props.model.getEmail());
}

function closeDropDown() {
    appStore.closeDropdowns();
}
</script>

<style scoped lang="scss">
    .project-member-item {

        &__menu {
            padding: 0 10px;
            position: relative;
            cursor: pointer;

            &__icon {

                &__content {
                    height: 32px;
                    width: 32px;
                    margin-left: auto;
                    margin-right: 0;
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
                top: 50px;
                right: 10px;
                background: var(--c-white);
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

    :deep(.primary) {
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
    }

    :deep(th) {
        max-width: 25rem;
    }

    @media screen and (width <= 940px) {

        :deep(th) {
            max-width: 10rem;
        }
    }
</style>
