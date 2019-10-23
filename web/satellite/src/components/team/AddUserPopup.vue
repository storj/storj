// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class='add-user-container' @keyup.enter="onAddUsersClick" @keyup.esc="onClose">
        <div class='add-user' id="addTeamMemberPopup">
            <div class="add-user__main">
                <div class='add-user__info-panel-container'>
                    <h2 class='add-user__info-panel-container__main-label-text'>Add Team Member</h2>
                    <AddMemberIcon/>
                </div>
                <div class='add-user__form-container'>
                    <p class='add-user__form-container__common-label' v-if="!formError">Email Address</p>
                    <div v-if="formError" class="add-user__form-container__label">
                        <ErrorIcon alt="Red error icon"/>
                        <p class="add-user__form-container__label__error">{{formError}}</p>
                    </div>
                    <div class="add-user__form-container__inputs-group" :class="{ 'scrollable': isInputsGroupScrollable }">
                        <div v-for="(input, index) in inputs"
                            class="add-user__form-container__inputs-group__item"
                            :key="index" >
                            <input
                                placeholder="email@example.com"
                                v-model="input.value"
                                class="no-error-input"
                                :class="{ 'error-input': input.error }"
                                @keyup="resetFormErrors(index)"
                            />
                            <DeleteFieldIcon
                                class="add-user__form-container__inputs-group__item__image"
                                @click="deleteInput(index)"
                            />
                        </div>
                    </div>
                    <div class="add-user-row">
                        <div @click='addInput' class="add-user-row__item" id="addUserButton">
                            <div :class="{ 'inactive-image': isMaxInputsCount }">
                                <AddFieldIcon class="add-user-row__item__image"/>
                            </div>
                            <p class="add-user-row__item__label" :class="{ 'inactive-label': isMaxInputsCount }">Add More</p>
                        </div>
                    </div>
                    <div class='add-user__form-container__button-container'>
                        <VButton
                            label='Cancel'
                            width='205px'
                            height='48px'
                            :on-press="onClose"
                            is-white="true"
                        />
                        <VButton
                            label='Add Team Members'
                            width='205px'
                            height='48px'
                            :on-press="onAddUsersClick"
                            :is-disabled="!isButtonActive"
                        />
                    </div>
                </div>
                <div class='add-user__close-cross-container' @click='onClose'>
                    <CloseCrossIcon/>
                </div>
            </div>
            <div class="notification-wrap">
                <AddMemberNotificationIcon class="notification-wrap__image"/>
                <div class="notification-wrap__text-area">
                    <p class="notification-wrap__text-area__text">If the team member you want to invite to join the project is still not on this Satellite, please share this link to the signup page and ask them to register here: <router-link target="_blank" exact to="/register" >{{registerPath}}</router-link></p>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang='ts'>
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import ErrorIcon from '@/../static/images/register/ErrorInfo.svg';
import AddFieldIcon from '@/../static/images/team/addField.svg';
import AddMemberIcon from '@/../static/images/team/addMember.svg';
import AddMemberNotificationIcon from '@/../static/images/team/addMemberNotification.svg';
import DeleteFieldIcon from '@/../static/images/team/deleteField.svg';

import { RouteConfig } from '@/router';
import { EmailInput } from '@/types/EmailInput';
import { APP_STATE_ACTIONS, PM_ACTIONS } from '@/utils/constants/actionNames';
import { validateEmail } from '@/utils/validation';

@Component({
    components: {
        VButton,
        AddMemberIcon,
        ErrorIcon,
        DeleteFieldIcon,
        AddFieldIcon,
        CloseCrossIcon,
        AddMemberNotificationIcon,
    },
})
export default class AddUserPopup extends Vue {
    private inputs: EmailInput[] = [new EmailInput(), new EmailInput(), new EmailInput()];
    private formError: string = '';
    private isLoading: boolean = false;

    private FIRST_PAGE = 1;

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
            const isEmail = validateEmail(element.value);

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
            const scrollableDiv: any = document.querySelector('.add-user__form-container__inputs-group');

            if (scrollableDiv) {
                const scrollableDivHeight = scrollableDiv.offsetHeight;
                scrollableDiv.scroll(0, -scrollableDivHeight);
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

        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_TEAM_MEMBERS);

        this.isLoading = false;
    }

    public addInput(): void {
        const inputsLength = this.inputs.length;
        if (inputsLength < 10) {
            this.inputs.push(new EmailInput());
        }
    }

    public deleteInput(index): void {
        if (this.inputs.length === 1) return;

        this.resetFormErrors(index);

        this.$delete(this.inputs, index);
    }

    public onClose(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_TEAM_MEMBERS);
    }

    public get isMaxInputsCount(): boolean {
        return this.inputs.length > 9;
    }

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

    public get isInputsGroupScrollable(): boolean {
        return this.inputs.length > 4;
    }

    private resetFormErrors(index): void {
        this.inputs[index].setError(false);
        if (!this.hasInputError()) {

            this.formError = '';
        }
    }

    private hasInputError(): boolean {
        return this.inputs.some((element: EmailInput) => {
            return element.error;
        });
    }
}
</script>

<style scoped lang='scss'>
    .add-user-container {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(134, 134, 148, 0.4);
        z-index: 1121;
        display: flex;
        justify-content: center;
        align-items: center;
        flex-direction: column;
        font-family: 'font_regular';
    }

    .add-user-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0 80px 0 50px;

        &__item {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__image {
                margin-right: 20px;
            }

            &__label {
                font-family: 'font_medium';
                font-size: 16px;
                margin-left: 0;
                padding-left: 0;
                margin-block-start: 0;
                margin-block-end: 0;
            }

            &:first-child {
                cursor: pointer;
                -webkit-user-select: none;
                -khtml-user-select: none;
                -moz-user-select: none;
                -ms-user-select: none;
                user-select: none;
            }
        }
    }

    .inactive-label {
        cursor: default;
        color: #DADDE5;
    }

    .error-input {
        border: 1px solid red !important;
    }

    .inactive-image {
        cursor: default;

        .add-user-row__item__image {

            &__rect {
                fill: #DADDE5;
            }

            &__path {
                fill: #ACB0BC;
            }
        }
    }

    .input-container.full-input {
        width: 100%;
    }

    .red {
        background-color: #EB5757;
    }

    .add-user {
        width: 100%;
        max-width: 1200px;
        height: auto;
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        position: relative;
        justify-content: center;

        &__main {
            border-top-left-radius: 6px;
            border-top-right-radius: 6px;
            display: flex;
            flex-direction: row;
            align-items: flex-start;
            position: relative;
            justify-content: center;
            background-color: #FFFFFF;
            padding: 80px 20px 80px 60px;
            width: calc(100% - 80px);
        }

        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            margin-right: 100px;
            padding: 0 50px;

            &__text {
                font-size: 16px;
                margin-top: 0;
                margin-bottom: 50px;
            }

            &__main-label-text {
                font-family: 'font_bold';
                font-size: 32px;
                line-height: 29px;
                color: #384B65;
                margin-top: 0;
                width: 107%;
            }
        }

        &__attention-text {
            font-size: 10px !important;
            line-height: 15px !important;
        }

        &__form-container {
            width: 100%;
            max-width: 600px;

            &__label {
                display: flex;
                flex-direction: row;
                padding-left: 50px;
                margin-bottom: 15px;

                &__error {
                    margin: 0;
                    padding-left: 10px;
                    color: #EB5757;
                }
            }

            &__inputs-group {
                max-height: 35vh;
                overflow-y: hidden;
                padding-left: 50px;
                padding-right: 50px;

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
                        border-color: rgba(56, 75, 101, 0.4);
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
                            fill: #2683FF;
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
                margin: 0 0 10px 0;
                font-family: 'font_medium';
                font-size: 16px;
                line-height: 25px;
                padding-left: 50px;
            }

            &__button-container {
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;
                margin-top: 30px;
                padding: 0 80px 0 50px;
            }
        }

        &__close-cross-container {
            display: flex;
            justify-content: center;
            align-items: center;
            position: absolute;
            right: 30px;
            top: 40px;
            height: 24px;
            width: 24px;
            cursor: pointer;

            &:hover .close-cross-svg-path {
                fill: #2683FF;
            }
        }
    }

    .notification-wrap {
        background-color: rgba(194, 214, 241, 1);
        height: 98px;
        width: calc(100% - 100px);
        display: flex;
        justify-content: flex-start;
        padding: 0 50px;
        align-items: center;
        border-bottom-left-radius: 6px;
        border-bottom-right-radius: 6px;

        &__image {
            margin-right: 40px;
            min-width: 40px;
        }

        &__text-area {
            display: flex;
            align-items: center;

            &__text {
                font-family: 'font_medium';
                font-size: 16px;
            }
        }
    }

    .scrollable {
        overflow-y: scroll;
    }

    @media screen and (max-width: 1025px) {
        .add-user {
            padding: 10px;
            max-width: 1000px;

            &__main {
                width: 100%;
                padding-right: 0;
                padding-left: 0;
            }

            &__info-panel-container {
                display: none;
            }

            &__form-container {
                max-width: 800px;
            }

            &-row__item {
                width: 80%;
            }
        }

        #addUserButton {
            justify-content: flex-start;

            .add-user-row__item__image {
                padding-right: 20px;
            }
        }
    }
</style>
