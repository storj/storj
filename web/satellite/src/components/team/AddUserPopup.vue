// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class='add-user-container' @keyup.enter="onAddUsersClick" @keyup.esc="onClose">
        <div class='add-user' id="addTeamMemberPopup">
            <div class="add-user__main">
                <div class='add-user__info-panel-container'>
                    <h2 class='add-user__info-panel-container__main-label-text'>Add Team Member</h2>
                    <div v-html='imageSource'></div>
                </div>
                <div class='add-user__form-container'>
                    <p v-if="!formError">Email Address</p>
                    <div v-if="formError" class="add-user__form-container__label">
                        <img src="../../../static/images/register/ErrorInfo.svg"/>
                        <p>{{formError}}</p>
                    </div>
                    <div :class="[inputs.length > 4 ? 'add-user__form-container__inputs-group scrollable' : 'add-user__form-container__inputs-group']">
                        <div v-for="(input, index) in inputs"
                            class="add-user__form-container__inputs-group__item"
                            :key="index" >
                                <input
                                    placeholder="test@mail.test"
                                    v-model="input.value"
                                    :class="[input.error ? 'error' : 'no-error']"
                                    @keyup="resetFormErrors(index)" />
                                <svg @click="deleteInput(index)" width="12" height="12" viewBox="0 0 12 12" fill="none" xmlns="http://www.w3.org/2000/svg">
                                    <path d="M11.7803 1.28033C12.0732 0.987437 12.0732 0.512563 11.7803 0.21967C11.4874 -0.0732233 11.0126 -0.0732233 10.7197 0.21967L11.7803 1.28033ZM0.21967 10.7197C-0.0732233 11.0126 -0.0732233 11.4874 0.21967 11.7803C0.512563 12.0732 0.987437 12.0732 1.28033 11.7803L0.21967 10.7197ZM1.28033 0.21967C0.987437 -0.0732233 0.512563 -0.0732233 0.21967 0.21967C-0.0732233 0.512563 -0.0732233 0.987437 0.21967 1.28033L1.28033 0.21967ZM10.7197 11.7803C11.0126 12.0732 11.4874 12.0732 11.7803 11.7803C12.0732 11.4874 12.0732 11.0126 11.7803 10.7197L10.7197 11.7803ZM10.7197 0.21967L0.21967 10.7197L1.28033 11.7803L11.7803 1.28033L10.7197 0.21967ZM0.21967 1.28033L10.7197 11.7803L11.7803 10.7197L1.28033 0.21967L0.21967 1.28033Z" fill="#AFB7C1"/>
                                </svg>
                        </div>
                    </div>
                    <div class="add-user-row">
                        <div @click='addInput' class="add-user-row__item" id="addUserButton">
                            <div :class="[isMaxInputsCount ? 'inactive-image' : '']">
                                <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
                                    <rect width="40" height="40" rx="20" fill="#2683FF" />
                                    <path d="M25 18.977V21.046H20.9722V25H19.0046V21.046H15V18.977H19.0046V15H20.9722V18.977H25Z" fill="white" />
                                </svg>
                            </div>
                            <p :class="[ isMaxInputsCount ? 'inactive-label' : '' ]">Add Another</p>
                        </div>
                    </div>
                    <div class='add-user__form-container__button-container'>
                        <Button label='Cancel' width='205px' height='48px' :onPress="onClose" isWhite="true"/>
                        <Button label='Add Team Members' width='205px' height='48px' :onPress="isButtonActive ? onAddUsersClick : () => {}" :isDisabled="!isButtonActive"/>
                    </div>
                </div>
                <div class='add-user__close-cross-container' @click='onClose'>
                    <svg width='16' height='16' viewBox='0 0 16 16' fill='none' xmlns='http://www.w3.org/2000/svg'>
                        <path d='M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z' fill='#384B65'/>
                    </svg>
                </div>
            </div>
            <div class="notification-wrap">
                <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect width="40" height="40" rx="10" fill="#2683FF"/>
                    <path d="M18.1489 17.043H21.9149V28H18.1489V17.043ZM20 12C20.5816 12 21.0567 12.1823 21.4255 12.5468C21.8085 12.8979 22 13.357 22 13.9241C22 14.4776 21.8085 14.9367 21.4255 15.3013C21.0567 15.6658 20.5816 15.8481 20 15.8481C19.4184 15.8481 18.9362 15.6658 18.5532 15.3013C18.1844 14.9367 18 14.4776 18 13.9241C18 13.357 18.1844 12.8979 18.5532 12.5468C18.9362 12.1823 19.4184 12 20 12Z" fill="#F5F6FA"/>
                </svg>
                <div class="notification-wrap__text">
                    <p>If the team member you want to invite to join the project is still not on this Satellite, please share this link to the signup page and ask them to register here: <router-link target="_blank" exact to="/register" >{{registerPath}}</router-link></p>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang='ts'>
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
    import { PM_ACTIONS, NOTIFICATION_ACTIONS, APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
    import { EmailInput } from '@/types/EmailInput';
    import { validateEmail } from '@/utils/validation';
    import ROUTES from '@/utils/constants/routerConstants';
    import { RequestResponse } from '@/types/response';

    @Component({
        components: {
            Button
        }
    })
    export default class AddUserPopup extends Vue {
        public imageSource: string = EMPTY_STATE_IMAGES.ADD_USER;
        private inputs: EmailInput[] = [new EmailInput(), new EmailInput(), new EmailInput()];
        private formError: string = '';
        private isLoading: boolean = false;

        public async onAddUsersClick(): Promise<void> {
            if (this.isLoading) {
                return;
            }

            this.isLoading = true;

            let length = this.inputs.length;
            let newInputsArray: EmailInput[] = [];
            let areAllEmailsValid = true;
            let emailArray: string[] = [];

            for (let i = 0; i < length; i++) {
                let element = this.inputs[i];
                let isEmail = validateEmail(element.value);

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
                let scrollableDiv: any = document.querySelector('.add-user__form-container__inputs-group');

                if (scrollableDiv) {
                    let scrollableDivHeight = scrollableDiv.offsetHeight;
                    scrollableDiv.scroll(0, -scrollableDivHeight);
                }
            }

            if (!areAllEmailsValid) {
                this.isLoading = false;

                return;
            }

            let result = await this.$store.dispatch(PM_ACTIONS.ADD, emailArray);
            if (!result.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Error during adding team members!');
                this.isLoading = false;

                return;
            }

            const response: RequestResponse<object> = await this.$store.dispatch(PM_ACTIONS.FETCH, { limit: 20, offset: 0 });

            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project members');
                this.isLoading = false;

                return;
            }

            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Members successfully added to project!');
            this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, '');

            const fetchMembersResponse: RequestResponse<object> = await this.$store.dispatch(PM_ACTIONS.FETCH);
            if (!fetchMembersResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project members');
            }

            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_TEAM_MEMBERS);

            this.isLoading = false;
        }

        public addInput(): void {
            let inputsLength = this.inputs.length;
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

            let length = this.inputs.length;

            for (let i = 0; i < length; i++) {
                if (this.inputs[i].value !== '') return true;
            }

            return false;
        }

        public get registerPath(): string {
            return location.host + ROUTES.REGISTER.path;
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

            &:first-child {
                cursor: pointer;
                -webkit-user-select: none;
                -khtml-user-select: none;
                -moz-user-select: none;
                -ms-user-select: none;
                user-select: none;

                svg {
                    margin-right: 20px;
                }

                p {
                    font-family: 'font_medium';
                    font-size: 16px;
                    margin-left: 0;
                    padding-left: 0;
                    margin-block-start: 0em;
                    margin-block-end: 0em;
                }
            }
        }
    }

    .inactive-label {
        color: #DADDE5;
    }

    .error {
        border: 1px solid red !important;
    }

    .inactive-image {

        svg {

            rect {
                fill: #DADDE5;
            }

            path {
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

    .text {
        margin: 0;
        margin-bottom: 0 !important;
        font-family: 'font_regular' !important;
        font-size: 16px;
        line-height: 25px;

        a {
            color: #2683FF;
            cursor: pointer;

            &:hover {
                text-decoration: underline;
            }
        }
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
                font-family: 'font_regular';
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
                margin-bottom: 10px;

                p {
                    margin: 0 !important;
                    padding-left: 10px !important;
                    color: #EB5757;
                }
            }

            &__inputs-group {
                max-height: 35vh;
                overflow-y: hidden;
                padding-left: 50px;
                padding-right: 50px;

                &.scrollable {
                    overflow-y: scroll;
                }

                &__item {
                    display: flex;
                    align-items: center;

                    input {
                        font-family: 'font_regular';
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

                    svg {
                        margin-bottom: 18px;
                        margin-left: 20px;
                        cursor: pointer;

                        &:hover path {
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

            p {
                margin: 0;
                margin-bottom: 10px;
                font-family: 'font_regular';
                font-size: 16px;
                line-height: 25px;
                padding-left: 50px;
            }

            a {
                font-family: 'font_medium';
                font-size: 16px;
                color: #2683FF;
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

            &:hover svg path {
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

        &__text {
            display: flex;
            align-items: center;

            p {
                font-family: 'font_medium';
                font-size: 16px;
                margin-left: 40px;

                span {
                    margin-right: 10px;
                }
            }

            a {
                cursor: pointer;
                color: #2683FF;

                &:hover {
                    text-decoration: underline;
                }
            }
        }
    }

    @media screen and (max-width: 1025px) {
        .add-user {
            padding: 10px;
            max-width: 1000px;

            &__main {
                width: 100%;
                padding-right: 0px;
                padding-left: 0px;
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

            svg {
                padding-right: 20px;
            }
        }
    }
</style>
