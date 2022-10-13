// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="add-user">
                <div class="add-user__main">
                    <div class="add-user__info-panel-container">
                        <h2 class="add-user__info-panel-container__title">Add Team Member</h2>
                        <img src="@/../static/images/team/addMember.jpg" alt="add team member image">
                    </div>
                    <div class="add-user__form-container">
                        <h2 class="add-user__form-container__title">Add Team Member</h2>
                        <p v-if="!formError" class="add-user__form-container__common-label">Email Address</p>
                        <div v-if="formError" class="add-user__form-container__label">
                            <ErrorIcon alt="Red error icon" />
                            <p class="add-user__form-container__label__error">{{ formError }}</p>
                        </div>
                        <div class="add-user__form-container__inputs-group">
                            <div
                                v-for="(input, index) in inputs"
                                :key="index"
                                class="add-user__form-container__inputs-group__item"
                            >
                                <input
                                    v-model="input.value"
                                    placeholder="email@example.com"
                                    class="no-error-input"
                                    :class="{ 'error-input': input.error }"
                                    @keyup="resetFormErrors(index)"
                                >
                                <DeleteFieldIcon
                                    class="add-user__form-container__inputs-group__item__image"
                                    @click="deleteInput(index)"
                                />
                            </div>
                        </div>
                        <div class="add-user-row">
                            <div id="add-user-button" class="add-user-row__item" @click="addInput">
                                <div :class="{ 'inactive-image': isMaxInputsCount }">
                                    <AddFieldIcon class="add-user-row__item__image" />
                                </div>
                                <p class="add-user-row__item__label" :class="{ 'inactive-label': isMaxInputsCount }">Add More</p>
                            </div>
                        </div>
                        <div class="add-user__form-container__button-container">
                            <VButton
                                label="Cancel"
                                width="100%"
                                height="48px"
                                :on-press="closeModal"
                                is-transparent="true"
                            />
                            <VButton
                                label="Add Team Members"
                                width="100%"
                                height="48px"
                                :on-press="onAddUsersClick"
                                :is-disabled="!isButtonActive"
                            />
                        </div>
                    </div>
                </div>
                <div class="notification-wrap">
                    <AddMemberNotificationIcon class="notification-wrap__image" />
                    <div class="notification-wrap__text-area">
                        <p class="notification-wrap__text-area__text">
                            If the team member you want to invite to join the project is still not on this Satellite, please
                            share this link to the signup page and ask them to register here:
                            <router-link target="_blank" rel="noopener noreferrer" exact to="/signup">
                                {{ registerPath }}
                            </router-link>
                        </p>
                    </div>
                </div>
            </div>
        </template>
    </VModal>
</template>

<script lang='ts'>
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { EmailInput } from '@/types/EmailInput';
import { PM_ACTIONS } from '@/utils/constants/actionNames';
import { Validator } from '@/utils/validation';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

import ErrorIcon from '@/../static/images/register/ErrorInfo.svg';
import AddFieldIcon from '@/../static/images/team/addField.svg';
import AddMemberNotificationIcon from '@/../static/images/team/addMemberNotification.svg';
import DeleteFieldIcon from '@/../static/images/team/deleteField.svg';

// @vue/component
@Component({
    components: {
        VButton,
        VModal,
        ErrorIcon,
        DeleteFieldIcon,
        AddFieldIcon,
        AddMemberNotificationIcon,
    },
})
export default class AddTeamMemberModal extends Vue {
    /**
     * Initial empty inputs set.
     */
    private inputs: EmailInput[] = [new EmailInput(), new EmailInput(), new EmailInput()];
    private formError = '';
    private isLoading = false;

    private FIRST_PAGE = 1;

    /**
     * Tries to add users related to entered emails list to current project.
     */
    public async onAddUsersClick(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;

        const length = this.inputs.length;
        const newInputsArray: EmailInput[] = [];
        let areAllEmailsValid = true;
        const emailArray: string[] = [];

        for (let i = 0; i < length; i++) {
            const element = this.inputs[i];
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

            this.formError = 'Field is required. Please enter a valid email address';
        }

        this.inputs = newInputsArray;

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
            this.isLoading = false;

            return;
        }

        if (emailArray.includes(this.$store.state.usersModule.email)) {
            await this.$notify.error(`Error during adding project members. You can't add yourself to the project`);
            this.isLoading = false;

            return;
        }

        try {
            await this.$store.dispatch(PM_ACTIONS.ADD, emailArray);
        } catch (error) {
            await this.$notify.error(`Error during adding project members. ${error.message}`);
            this.isLoading = false;

            return;
        }

        await this.$notify.success('Members successfully added to project!');
        this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, '');

        try {
            await this.$store.dispatch(PM_ACTIONS.FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project members. ${error.message}`);
        }

        this.closeModal();

        this.isLoading = false;
    }

    /**
     * Adds additional email input.
     */
    public addInput(): void {
        const inputsLength = this.inputs.length;
        if (inputsLength < 10) {
            this.inputs.push(new EmailInput());
        }
    }

    /**
     * Deletes selected email input from list.
     * @param index
     */
    public deleteInput(index: number): void {
        if (this.inputs.length === 1) return;

        this.resetFormErrors(index);

        this.$delete(this.inputs, index);
    }

    /**
     * Closes modal.
     */
    public closeModal(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_ADD_TEAM_MEMBERS_MODAL);
    }

    /**
     * Indicates if emails count reached maximum.
     */
    public get isMaxInputsCount(): boolean {
        return this.inputs.length > 9;
    }

    /**
     * Indicates if add button is active.
     * Active when no errors and at least one input is not empty.
     */
    public get isButtonActive(): boolean {
        if (this.formError) return false;

        const length = this.inputs.length;

        for (let i = 0; i < length; i++) {
            if (this.inputs[i].value !== '') return true;
        }

        return false;
    }

    public get registerPath(): string {
        return location.host + RouteConfig.Register.path;
    }

    /**
     * Removes error for selected input.
     */
    private resetFormErrors(index): void {
        this.inputs[index].setError(false);
        if (!this.hasInputError) {

            this.formError = '';
        }
    }

    /**
     * Indicates if at least one input has error.
     */
    private get hasInputError(): boolean {
        return this.inputs.some((element: EmailInput) => {
            return element.error;
        });
    }
}
</script>

<style scoped lang='scss'>
    .add-user {
        width: 100%;
        max-width: 1200px;
        display: flex;
        flex-direction: column;
        justify-content: center;
        font-family: 'font_regular', sans-serif;

        &__main {
            border-radius: 6px 6px 0 0;
            display: flex;
            align-items: flex-start;
            justify-content: center;
            background-color: #fff;
            padding: 80px 24px;
            width: calc(100% - 48px);

            @media screen and (max-width: 950px) {
                padding: 48px 24px;
            }
        }

        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            margin-right: 150px;

            @media screen and (max-width: 950px) {
                display: none;
            }

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 29px;
                color: #384b65;
                margin: 0 0 90px;
                width: 130%;
                text-align: end;
            }
        }

        &__attention-text {
            font-size: 10px !important;
            line-height: 15px !important;
        }

        &__form-container {
            width: 100%;
            max-width: 600px;

            @media screen and (max-width: 950px) {
                max-width: unset;
            }

            &__title {
                display: none;
                font-family: 'font_bold', sans-serif;
                font-size: 28px;
                color: #384b65;
                margin-bottom: 20px;
                text-align: left;

                @media screen and (max-width: 950px) {
                    display: block;
                }
            }

            &__label {
                display: flex;
                padding-left: 50px;
                margin-bottom: 15px;

                @media screen and (max-width: 950px) {
                    padding: 0;
                }

                @media screen and (max-width: 550px) {

                    svg {
                        display: none;
                    }
                }

                &__error {
                    margin-left: 10px;
                    color: #eb5757;
                    text-align: left;

                    @media screen and (max-width: 550px) {
                        margin: 0;
                    }
                }
            }

            &__inputs-group {
                padding: 3px 50px 0;

                @media screen and (max-width: 950px) {
                    padding: 0;
                }

                &__item {
                    display: flex;
                    align-items: center;

                    .no-error-input {
                        font-size: 16px;
                        line-height: 21px;
                        resize: none;
                        margin-bottom: 18px;
                        height: 48px;
                        width: 100%;
                        text-indent: 20px;
                        border-color: rgb(56 75 101 / 40%);
                        border-radius: 6px;

                        &:last-child {
                            margin-bottom: 0;
                        }
                    }

                    &__image {
                        margin-bottom: 18px;
                        margin-left: 20px;
                        cursor: pointer;

                        &:hover .delete-input-svg-path {
                            fill: #2683ff;
                        }
                    }
                }
            }

            .full-input {
                margin-bottom: 18px;

                &:last-child {
                    margin-bottom: 0;
                }
            }

            &__common-label {
                margin: 0 0 10px;
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 25px;
                padding-left: 50px;
                text-align: left;

                @media screen and (max-width: 950px) {
                    padding-left: unset;
                }
            }

            &__button-container {
                display: flex;
                justify-content: space-between;
                align-items: center;
                margin-top: 30px;
                padding: 0 80px 0 50px;
                column-gap: 20px;

                @media screen and (max-width: 950px) {
                    padding: 0;
                }

                @media screen and (max-width: 420px) {
                    flex-direction: column-reverse;
                    row-gap: 10px;
                    column-gap: unset;
                }
            }
        }
    }

    .add-user-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0 80px 0 50px;

        @media screen and (max-width: 950px) {
            padding: 0;
        }

        &__item {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__image {
                margin-right: 20px;
            }

            &__label {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                margin-left: 0;
                padding-left: 0;
                margin-block-start: 0;
                margin-block-end: 0;
            }

            &:first-child {
                cursor: pointer;
            }
        }
    }

    .inactive-label {
        cursor: default;
        color: #dadde5;
    }

    .error-input {
        border: 1px solid red !important;
    }

    .inactive-image {
        cursor: default;

        .add-user-row__item__image {

            &__rect {
                fill: #dadde5;
            }

            &__path {
                fill: #acb0bc;
            }
        }
    }

    .input-container.full-input {
        width: 100%;
    }

    .red {
        background-color: #eb5757;
    }

    .notification-wrap {
        background-color: rgb(194 214 241 / 100%);
        height: 98px;
        display: flex;
        justify-content: flex-start;
        padding: 0 50px;
        align-items: center;
        border-radius: 0 0 6px 6px;

        @media screen and (max-width: 950px) {
            height: unset;
            padding: 24px;
        }

        &__image {
            margin-right: 40px;
            min-width: 40px;

            @media screen and (max-width: 500px) {
                display: none;
            }
        }

        &__text-area {
            display: flex;
            align-items: center;

            &__text {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                text-align: left;

                @media screen and (max-width: 500px) {
                    font-size: 14px;
                }
            }
        }
    }

    :deep(.container) {
        padding: 0 10px;
    }

    @media screen and (max-width: 500px) {

        :deep(.container) {
            padding: 0;
        }
    }
</style>
