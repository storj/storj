// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class='add-user-container'>
        <div class='add-user' id="addTeamMemberPopup">
            <div class='add-user__info-panel-container'>
                <h2 class='add-user__info-panel-container__main-label-text'>Add New User</h2>
                <div v-html='imageSource'></div>
            </div>
            <div class='add-user__form-container'>
                <p v-if="!formError">Email Address</p>
                <div v-if="formError" class="add-user__form-container__label">
                    <img src="../../../static/images/register/ErrorInfo.svg"/>
                    <p>{{formError}}</p>
                </div>
                <div class="add-user__form-container__inputs-group">
                    <div v-for="(input, index) in inputs" 
                        class="add-user__form-container__inputs-group__item" 
                        v-bind:key="index" >
                        <input 
                            placeholder="test@test.net" 
                            v-model="input.value" 
                            v-bind:class="[input.error ? 'error' : 'no-error']"
                            v-on:keyup="resetFormErrors(index)" />
                        <span v-html="imageDeleteUser" @click="deleteInput(index)"></span>
                    </div>
                </div>
                <div class="add-user-row">
                    <div v-on:click='addInput' class="add-user-row__item" id="addUserButton">
                        <div v-bind:class="[isMaxInputsCount ? 'inactive-image' : '']">
                            <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <rect width="40" height="40" rx="20" fill="#2683FF"/>
                                <path d="M25 18.977V21.046H20.9722V25H19.0046V21.046H15V18.977H19.0046V15H20.9722V18.977H25Z" fill="white"/>
                            </svg>
                        </div>
                        <p v-bind:class="[isMaxInputsCount ? 'inactive-label' : '']">Add Another</p>
                    </div>
                    <div class="add-user-row__item">
                        <p>You can add only those user who already registred on Storj Satellite</p>
                    </div>
                </div>
                <div class='add-user__form-container__button-container'>
                    <Button label='Cancel' width='205px' height='48px' :onPress="onClose" isWhite/>
                    <Button label='Add Users' width='205px' height='48px' :onPress="isButtonActive ? onAddUsersClick : () => {}" :isDisabled="!isButtonActive"/>
                </div>
            </div>
            <div class='add-user__close-cross-container'>
                <svg width='16' height='16' viewBox='0 0 16 16' fill='none' xmlns='http://www.w3.org/2000/svg' v-on:click='onClose'>
                    <path d='M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z' fill='#384B65'/>
                </svg>
            </div>
        </div>
    </div>
</template>

<script lang='ts'>
import { Component, Vue } from 'vue-property-decorator';
import Button from '@/components/common/Button.vue';
import { removeToken } from '@/utils/tokenManager';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
import { PM_ACTIONS, NOTIFICATION_ACTIONS, APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { EmailInput } from '@/types/EmailInput';
import { validateEmail } from '@/utils/validation';

@Component(
    {
        data: function() {
            return {
                inputs: [new EmailInput(), new EmailInput(), new EmailInput()],
                formError: '',
                imageSource: EMPTY_STATE_IMAGES.ADD_USER,
                imageDeleteUser: EMPTY_STATE_IMAGES.DELETE_USER,
            };
        },
        methods: {
            onAddUsersClick: async function() {
                let length = this.$data.inputs.length;
                let newInputsArray: any[] = [];
                let areAllEmailsValid = true;
                let emailArray: string[] = [];

                for (let i = 0; i < length; i++) {
                    let element = this.$data.inputs[i];
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

                    this.$data.formError = 'Field is required. Please enter a valid email address';
                }

                this.$data.inputs = newInputsArray;

                if (length > 3) {
                    let scrollableDiv: any = document.querySelector('.add-user__form-container__inputs-group');

                    if (scrollableDiv) {
                        let scrollableDivHeight = scrollableDiv.offsetHeight;
                        scrollableDiv.scroll(0, -scrollableDivHeight);
                    }
                }

                if (!areAllEmailsValid) return;
                
                let result = await this.$store.dispatch(PM_ACTIONS.ADD, emailArray);
                
                if (!result.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Error during adding team members!');

                    return;
                }

                const response = await this.$store.dispatch(PM_ACTIONS.FETCH, { limit: 20, offset: 0 });

                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project members');
            
                    return;
                }

                this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Members successfully added to project!');
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_TEAM_MEMBERS);
            },
            addInput: function(): void {
                let inputsLength = this.$data.inputs.length;

                if (inputsLength < 10) {
                    this.$data.inputs.push(new EmailInput());
                }
            },
            deleteInput: function(index): void {
                if (this.$data.inputs.length === 1) return;

                this.$delete(this.$data.inputs, index);
            },
            resetFormErrors: function(index): void {
                this.$data.formError = '';
                this.$data.inputs[index].setError(false);
            },
            onClose: function(): void {
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_TEAM_MEMBERS);
            },
         },
        computed: {
            isMaxInputsCount: function(): boolean {
                return this.$data.inputs.length > 9;
            },
            isButtonActive: function(): boolean {
                if (this.$data.formError) return false;

                let length = this.$data.inputs.length;

                for (let i = 0; i < length; i++) {
                    if (this.$data.inputs[i].value !== '') return true;
                }

                return false;
            }
        },
        components: {
            Button
        }
    }
)

export default class AddUserPopup extends Vue {}
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
    }

    .add-user-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0 81px 0 47px;

        &__item {
            display: flex;
            align-items: center;
            justify-content: space-between;
            width: 60%;

            &:first-child {
                width: 36%;
                cursor: pointer;
                -webkit-user-select: none;
                -khtml-user-select: none;
                -moz-user-select: none;
                -ms-user-select: none;
                user-select: none;

                p {
                    margin: 0 !important;
                    font-family: 'montserrat_medium';
                    font-size: 16px;
                    margin-left: 0;
                    padding-left: 0;
                }
            }

            &:last-child {
                p {
                    font-size: 14px !important;
                    margin: 0 !important;
                    text-align: right;
                    padding-left: 0;
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
        font-family: 'montserrat_regular' !important;
        font-size: 16px;
        line-height: 25px;
    }

    .add-user {
        width: 100%;
        max-width: 1040px;
        max-height: 80vh;
        background-color: #FFFFFF;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        position: relative;
        justify-content: center;
        padding: 80px 20px 80px 60px;

        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            margin-right: 100px;

            &__main-label-text {
                font-family: 'montserrat_bold';
                font-size: 32px;
                line-height: 39px;
                color: #384B65;
                margin-bottom: 60px;
                margin-top: 0;
            }
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
                overflow-y: scroll;
                padding-left: 50px;
                padding-right: 50px;

                &__item {
                    display: flex;
                    align-items: center;

                    input {
                        font-family: 'montserrat_regular';
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
                    
                    span {
                        margin-bottom: 18px;
                        margin-left: 20px;
                        cursor: pointer;
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
                font-family: 'montserrat_regular';
                font-size: 16px;
                line-height: 25px;
                padding-left: 50px;

                &:nth-child(2) {
                    margin-top: 20px;
                }
            }

            a {
                font-family: 'montserrat_medium';
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
            align-items: flex-start;
            position: absolute;
            right: 30px;
            top: 40px;
            svg {
                cursor: pointer;
            }
        }
    }

    @media screen and (max-width: 720px) {
        .add-user {
            padding: 10px;

            &__info-panel-container {
                display: none;

            }

            &__form-container {

                &__button-container {
                    width: 100%;
                }
            }
        }
    }
</style>
