// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <TeamMembersIcon />
                    <h1 class="modal__header__title">Invite team members</h1>
                </div>

                <p class="modal__info">
                    Add team members to contribute to this project.
                </p>

                <div class="modal__input-group">
                    <VInput
                        v-for="(_, index) in inputs"
                        :key="index"
                        class="modal__input-group__item"
                        label="Email"
                        height="38px"
                        placeholder="email@email.com"
                        role-description="email"
                        :error="formError"
                        :max-symbols="72"
                        @setData="(str) => setInput(index, str)"
                    />
                </div>

                <div class="modal__more">
                    <div
                        tabindex="0"
                        class="modal__more__button"
                        @click.stop="addInput"
                    >
                        <AddCircleIcon class="modal__more__button__icon" :class="{ inactive: isMaxInputsCount }" />
                        <span class="modal__more__button__label" :class="{ inactive: isMaxInputsCount }">Add more</span>
                    </div>
                </div>

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
                        label="Invite"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="onAddUsersClick"
                        :is-disabled="!isButtonActive"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang='ts'>
import { computed, ref } from 'vue';

import { EmailInput } from '@/types/EmailInput';
import { Validator } from '@/utils/validation';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';
import VInput from '@/components/common/VInput.vue';

import TeamMembersIcon from '@/../static/images/team/teamMembers.svg';
import AddCircleIcon from '@/../static/images/common/addCircle.svg';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const pmStore = useProjectMembersStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const notify = useNotify();

const FIRST_PAGE = 1;

const inputs = ref<EmailInput[]>([new EmailInput()]);
const formError = ref<string>('');
const isLoading = ref<boolean>(false);

/**
 * Indicates if at least one input has error.
 */
const hasInputError = computed((): boolean => {
    return inputs.value.some((element: EmailInput) => {
        return element.error;
    });
});

/**
 * Indicates if emails count reached maximum.
 */
const isMaxInputsCount = computed((): boolean => {
    return inputs.value.length > 9;
});

/**
 * Indicates if add button is active.
 * Active when no errors and at least one input is not empty.
 */
const isButtonActive = computed((): boolean => {
    if (formError.value) return false;

    const length = inputs.value.length;

    for (let i = 0; i < length; i++) {
        if (inputs.value[i].value !== '') return true;
    }

    return false;
});

function setInput(index: number, str: string) {
    resetFormErrors(index);
    inputs.value[index].value = str;
}

/**
 * Tries to add users related to entered emails list to current project.
 */
async function onAddUsersClick(): Promise<void> {
    if (isLoading.value) return;

    isLoading.value = true;

    const length = inputs.value.length;
    const newInputsArray: EmailInput[] = [];
    let areAllEmailsValid = true;
    const emailArray: string[] = [];

    inputs.value.forEach(elem => elem.value = elem.value.trim());

    for (let i = 0; i < length; i++) {
        const element = inputs.value[i];
        const isEmail = Validator.email(element.value);

        if (isEmail) {
            emailArray.push(element.value);
        }

        if (isEmail || element.value === '') {
            element.setError(false);
            newInputsArray.push(element);

            continue;
        }

        element.setError(true);
        newInputsArray.unshift(element);
        areAllEmailsValid = false;

        formError.value = 'Please enter a valid email address';
    }

    inputs.value = [...newInputsArray];

    if (length > 3) {
        const scrollableDiv = document.querySelector('.add-user__form-container__inputs-group');
        if (scrollableDiv) {
            const scrollableDivHeight = scrollableDiv.getAttribute('offsetHeight');
            if (scrollableDivHeight) {
                scrollableDiv.scroll(0, -scrollableDivHeight);
            }
        }
    }

    if (!areAllEmailsValid) {
        isLoading.value = false;
        return;
    }

    if (emailArray.includes(usersStore.state.user.email)) {
        notify.error(`Error adding project members. You can't add yourself to the project`, AnalyticsErrorEventSource.ADD_PROJECT_MEMBER_MODAL);
        isLoading.value = false;

        return;
    }

    try {
        await pmStore.inviteMembers(emailArray, projectsStore.state.selectedProject.id);
    } catch (error) {
        error.message = `Error adding project members. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.ADD_PROJECT_MEMBER_MODAL);
        isLoading.value = false;

        return;
    }

    analyticsStore.eventTriggered(AnalyticsEvent.PROJECT_MEMBERS_INVITE_SENT);
    notify.notify('Invites sent!');
    pmStore.setSearchQuery('');

    try {
        await pmStore.getProjectMembers(FIRST_PAGE, projectsStore.state.selectedProject.id);
    } catch (error) {
        error.message = `Unable to fetch project members. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.ADD_PROJECT_MEMBER_MODAL);
    }

    closeModal();

    isLoading.value = false;
}

/**
 * Adds additional email input.
 */
function addInput(): void {
    const inputsLength = inputs.value.length;
    if (inputsLength < 10) {
        inputs.value.push(new EmailInput());
    }
}

/**
 * Closes modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}

/**
 * Removes error for selected input.
 */
function resetFormErrors(index: number): void {
    inputs.value[index].setError(false);
    if (!hasInputError.value) {
        formError.value = '';
    }
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

        &__input-group {

            &__item {
                border-bottom: 1px solid var(--c-grey-2);
                padding-bottom: 16px;
                margin-bottom: 16px;
            }
        }

        &__more {
            border-bottom: 1px solid var(--c-grey-2);
            padding-bottom: 16px;
            margin-bottom: 16px;

            &__button {
                width: fit-content;
                display: flex;
                column-gap: 5px;
                align-items: flex-end;
                cursor: pointer;

                &__icon {
                    width: 18px;
                    height: 18px;

                    &.inactive {

                        :deep(path) {
                            fill: var(--c-grey-5);
                        }
                    }

                    :deep(path) {
                        fill: var(--c-blue-3);
                    }
                }

                &__label {
                    font-family: 'font_regular', sans-serif;
                    font-size: 16px;
                    text-decoration: underline;
                    text-align: center;
                    color: var(--c-blue-3);

                    &.inactive {
                        color: var(--c-grey-5);
                    }
                }
            }
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
