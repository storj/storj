// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="team-header-container">
        <div class="team-header-container__title-area">
            <div class="team-header-container__title-area__titles">
                <span class="team-header-container__title-area__titles__title" aria-roledescription="title">Team</span>
                <VInfo class="team-header-container__title-area__titles__info-button">
                    <template #icon>
                        <InfoIcon />
                    </template>
                    <template #message>
                        <p class="team-header-container__title-area__info-button__message">
                            The only project role currently available is Admin, which gives full access to the project.
                        </p>
                    </template>
                </VInfo>
                <p class="team-header-container__title-area__titles__subtitle" aria-roledescription="subtitle">
                    Manage the team members of "{{ projectName }}"
                </p>
            </div>
            <VButton
                class="team-header-container__title-area__button"
                label="Add Members"
                width="160px"
                height="40px"
                font-size="13px"
                border-radius="8px"
                :on-press="toggleAddTeamMembersModal"
                icon="add"
                :is-disabled="isAddButtonDisabled"
            />
        </div>

        <div class="team-header-container__divider" />

        <div class="team-header-container__wrapper">
            <VSearch
                ref="searchInput"
                class="team-header-container__wrapper__search"
                :search="processSearchQuery"
            />
            <div v-if="selectedEmailsLength" class="team-header-container__wrapper__right">
                <span class="team-header-container__wrapper__right__selected-text">
                    {{ selectedEmailsLength }} user{{ selectedEmailsLength !== 1 ? 's' : '' }} selected
                </span>
                <div class="team-header-container__wrapper__right__buttons">
                    <VButton
                        class="team-header-container__wrapper__right__buttons__button"
                        label="Delete"
                        border-radius="8px"
                        font-size="12px"
                        is-white
                        icon="trash"
                        :on-press="toggleRemoveTeamMembersModal"
                    />
                    <VButton
                        v-if="resendInvitesShown"
                        class="team-header-container__wrapper__right__buttons__button"
                        label="Resend invite"
                        border-radius="8px"
                        font-size="12px"
                        is-white
                        icon="upload"
                        :on-press="resendInvites"
                        :is-disabled="isLoading"
                    />
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify } from '@/utils/hooks';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useLoading } from '@/composables/useLoading';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VInfo from '@/components/common/VInfo.vue';
import VButton from '@/components/common/VButton.vue';
import VSearch from '@/components/common/VSearch.vue';

import InfoIcon from '@/../static/images/team/infoTooltip.svg';

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const appStore = useAppStore();
const pmStore = useProjectMembersStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const props = withDefaults(defineProps<{
    isAddButtonDisabled: boolean;
}>(), {
    isAddButtonDisabled: false,
});

const FIRST_PAGE = 1;

const searchInput = ref<InstanceType<typeof VSearch> | null>(null);

/**
 * Returns the name of the selected project from store.
 */
const projectName = computed((): string => {
    return projectsStore.state.selectedProject.name;
});

const selectedEmailsLength = computed((): number => {
    return pmStore.state.selectedProjectMembersEmails.length;
});

const resendInvitesShown = computed((): boolean => {
    const expired = pmStore.state.page.projectInvitations.filter(invite => invite.expired);
    return pmStore.state.selectedProjectMembersEmails.every(email => {
        return expired.some(invite => invite.email === email);
    });
});

/**
 * Opens add team members modal.
 */
function toggleAddTeamMembersModal(): void {
    appStore.updateActiveModal(MODALS.addTeamMember);
}

function toggleRemoveTeamMembersModal(): void {
    appStore.updateActiveModal(MODALS.removeTeamMember);
}

/**
 * Fetches team members of current project depends on search query.
 * @param search
 */
async function processSearchQuery(search: string): Promise<void> {
    // avoid infinite loop due to listener on pmStore.setSearchQuery('') itself indirectly calling pmStore.setSearchQuery('')
    if (pmStore.getSearchQuery() !== search) {
        pmStore.setSearchQuery(search);
    }
    try {
        const id = projectsStore.state.selectedProject.id;
        if (!id) {
            return;
        }
        await pmStore.getProjectMembers(FIRST_PAGE, id);
    } catch (error) {
        notify.error(`Unable to fetch project members. ${error.message}`, AnalyticsErrorEventSource.PROJECT_MEMBERS_HEADER);
    }
}

/**
 * resendInvites resends project member invitations.
 * It expects that all of the selected project member emails belong to expired invitations.
 */
async function resendInvites(): Promise<void> {
    await withLoading(async () => {
        analyticsStore.eventTriggered(AnalyticsEvent.RESEND_INVITE_CLICKED);

        try {
            await pmStore.inviteMembers(pmStore.state.selectedProjectMembersEmails, projectsStore.state.selectedProject.id);
            notify.success('Invites re-sent!');
        } catch (error) {
            error.message = `Unable to resend project invitations. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_MEMBERS_HEADER);
        }

        try {
            await pmStore.refresh();
        } catch (error) {
            notify.error(`Unable to fetch project members. ${error.message}`, AnalyticsErrorEventSource.PROJECT_MEMBERS_HEADER);
        }
    });
}

/**
 * Lifecycle hook after initial render.
 * Set up listener to clear search bar.
 */
onMounted((): void => {
    if (configStore.state.config.allProjectsDashboard && !projectsStore.state.selectedProject.id) {
        // navigation back to the all projects dashboard is done in ProjectMembersArea.vue
        return;
    }

    pmStore.$onAction(({ name, after, args }) => {
        if (name === 'setSearchQuery' && args[0] === '') {
            after((_) => searchInput.value?.clearSearch());
        }
    });
});

/**
 * Lifecycle hook before component destruction.
 * Clears search query for team members page.
 */
onBeforeUnmount((): void => {
    pmStore.setSearchQuery('');
});
</script>

<style scoped lang="scss">
    .team-header-container {

        &__title-area {
            width: 100%;
            display: flex;
            justify-content: space-between;
            align-items: center;

            @media screen and (width <= 1150px) {
                flex-direction: column;
                align-items: flex-start;
                justify-content: flex-start;
                row-gap: 10px;

                &__button {
                    width: 100% !important;
                }
            }

            &__titles {

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-weight: 600;
                    font-size: 28px;
                    line-height: 34px;
                    color: #232b34;
                    text-align: left;
                    display: inline;
                }

                &__subtitle {
                    font-size: 14px;
                    line-height: 20px;
                    font-weight: bold;
                    margin-top: 10px;
                }

                &__info-button {
                    max-height: 20px;
                    cursor: pointer;
                    margin-left: 10px;
                    display: inline;

                    &__message {
                        color: #586c86;
                        font-family: 'font_regular', sans-serif;
                        font-size: 16px;
                        line-height: 18px;
                    }
                }
            }
        }

        &__divider {
            width: 100%;
            height: 1px;
            background: #dadfe7;
            margin: 24px 0;
        }

        &__wrapper {
            position: relative;
            display: flex;
            align-items: center;
            justify-content: space-between;

            @media screen and (width <= 1150px) {
                flex-direction: column;
                align-items: flex-start;
                justify-content: flex-start;
                row-gap: 10px;
            }

            &__search {
                position: static;
            }

            &__right {
                display: flex;
                align-items: center;
                gap: 20px;

                @media screen and (width <= 1150px) {
                    width: 100%;
                    flex-direction: column-reverse;
                    align-items: flex-start;
                    gap: 8px;
                }

                &__selected-text {
                    color: rgb(0 0 0 / 60%);
                    font-family: 'font_regular', sans-serif;
                    font-size: 14px;
                    line-height: 24px;
                }

                &__buttons {
                    display: flex;
                    gap: 14px;

                    @media screen and (width <= 1150px) {
                        width: 100%;
                    }

                    &__button {
                        padding: 8px 12px;

                        @media screen and (width <= 1150px) {
                            padding: 12px;
                        }

                        :deep(.label) {
                            color: #56606D !important;
                        }

                        :deep(path) {
                            fill: #56606D !important;
                        }
                    }
                }
            }
        }
    }

    :deep(.info__box__message) {
        min-width: 300px;
    }
</style>
