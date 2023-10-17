// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <TeamMembersIcon />
                    <h1 class="modal__header__title">
                        {{ isPaidTier ? 'Invite team member' : 'Upgrade to Pro' }}
                    </h1>
                </div>

                <p class="modal__info">
                    <template v-if="isPaidTier">
                        Add a team member to contribute to this project.
                    </template>
                    <template v-else>
                        Upgrade now to unlock collaboration and bring your team together in this project.
                    </template>
                </p>

                <VInput
                    v-if="isPaidTier"
                    class="modal__input"
                    label="Email"
                    height="38px"
                    placeholder="email@email.com"
                    role-description="email"
                    :error="typeof formError === 'string' ? formError : undefined"
                    :max-symbols="72"
                    @setData="str => email = str.trim()"
                />

                <div class="modal__buttons">
                    <VButton
                        label="Cancel"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :is-transparent="true"
                        :on-press="closeModal"
                    />
                    <VButton
                        :label="isPaidTier ? 'Invite' : 'Upgrade'"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="onPrimaryClick"
                        :is-disabled="!!formError || isLoading"
                    >
                        <template v-if="!isPaidTier" #icon-right>
                            <ArrowIcon />
                        </template>
                    </VButton>
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang='ts'>
import { computed, ref } from 'vue';

import { Validator } from '@/utils/validation';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useLoading } from '@/composables/useLoading';
import { MODALS } from '@/utils/constants/appStatePopUps';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';
import VInput from '@/components/common/VInput.vue';

import TeamMembersIcon from '@/../static/images/team/teamMembers.svg';
import ArrowIcon from '@/../static/images/onboardingTour/arrowRight.svg';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const pmStore = useProjectMembersStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const FIRST_PAGE = 1;

const email = ref<string>('');

/**
 * Returns a boolean indicating whether the email is invalid
 * or a message describing the validation error.
 */
const formError = computed<string | boolean>(() => {
    if (!isPaidTier.value) return false;
    if (!email.value) return true;
    if (email.value.toLocaleLowerCase() === usersStore.state.user.email.toLowerCase()) {
        return `You can't add yourself to the project.`;
    }
    if (!Validator.email(email.value)) {
        return 'Please enter a valid email address.';
    }
    return false;
});

/**
 * Returns user's paid tier status from store.
 */
const isPaidTier = computed<boolean>(() => {
    return usersStore.state.user.paidTier;
});

/**
 * Handles primary button click.
 */
async function onPrimaryClick(): Promise<void> {
    if (!isPaidTier.value) {
        appStore.updateActiveModal(MODALS.upgradeAccount);
        return;
    }

    await withLoading(async () => {
        try {
            await pmStore.inviteMember(email.value, projectsStore.state.selectedProject.id);
        } catch (error) {
            error.message = `Error inviting project member. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.ADD_PROJECT_MEMBER_MODAL);
            return;
        }

        analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_MEMBERS_INVITE_SENT);
        notify.notify('Invite sent!');
        pmStore.setSearchQuery('');

        try {
            await pmStore.getProjectMembers(FIRST_PAGE, projectsStore.state.selectedProject.id);
        } catch (error) {
            error.message = `Unable to fetch project members. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.ADD_PROJECT_MEMBER_MODAL);
        }

        closeModal();
    });
}

/**
 * Closes modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}
</script>

<style scoped lang='scss'>
    .modal {
        width: 346px;
        padding: 32px;

        @media screen and (width <= 460px) {
            width: 280px;
            padding: 16px;
        }

        &__header {
            display: flex;
            align-items: center;
            padding-bottom: 16px;
            margin-bottom: 16px;
            border-bottom: 1px solid var(--c-grey-2);

            @media screen and (width <= 460px) {
                flex-direction: column;
                align-items: flex-start;
            }

            &__title {
                margin-left: 16px;
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 31px;
                letter-spacing: -0.02em;
                color: var(--c-black);
                text-align: left;

                @media screen and (width <= 460px) {
                    margin: 10px 0 0;
                }
            }
        }

        &__info {
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            line-height: 19px;
            color: var(--c-black);
            border-bottom: 1px solid var(--c-grey-2);
            text-align: left;
            padding-bottom: 16px;
            margin-bottom: 16px;
        }

        &__input {
            border-bottom: 1px solid var(--c-grey-2);
            padding-bottom: 16px;
            margin-bottom: 16px;
        }

        &__buttons {
            display: flex;
            column-gap: 10px;
            margin-top: 10px;
            width: 100%;

            @media screen and (width <= 500px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 10px;
            }
        }
    }

    :deep(.label-container__main__label) {
        font-size: 14px;
    }

    :deep(.label-container__main__error) {
        font-size: 14px;
    }

    :deep(.input-container) {
        margin-top: 0;
    }
</style>
