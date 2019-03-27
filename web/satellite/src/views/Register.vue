// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="register-container" v-on:keyup.enter="onCreateClick">
        <div v-bind:class="loadingClassName">
            <img src="../../static/images/register/Loading.gif">
        </div>
        <img class="planet" src="../../static/images/Mars.png" alt="" >
        <div class="register-container__wrapper">
            <div class="register-container__header">
                <img class="register-container__logo" src="../../static/images/Logo.svg" alt="logo" v-on:click="onLogoClick">
                <div class="register-container__register-button" v-on:click.prevent="onLoginClick">
                    <p>Login</p>
                </div>
            </div>
            <div class="register-area-wrapper">
                <div class="register-area">
                    <div class="register-area__title-container">
                        <h1>Sign Up to Storj</h1>
                        <p>Satellite:<b>Mars</b></p>
                    </div>
                    <HeaderlessInput
                        class="full-input"
                        label="Full name"
                        placeholder="Enter Full Name"
                        :error="fullNameError"
                        @setData="setFullName"
                        width="100%"
                        height="46px"
                        isWhite>
                    </HeaderlessInput>
                    <HeaderlessInput
                        class="full-input"
                        label="Short Name"
                        placeholder="Enter Short Name"
                        @setData="setShortName"
                        width="100%"
                        height="46px"
                        isWhite>
                    </HeaderlessInput>
                    <HeaderlessInput
                        class="full-input"
                        label="Email"
                        placeholder="Enter Email"
                        :error="emailError"
                        @setData="setEmail"
                        width="100%"
                        height="46px"
                        isWhite>
                    </HeaderlessInput>
                    <div class="register-input">
                        <HeaderlessInput
                            class="full-input"
                            label="Password"
                            placeholder="Enter Password"
                            :error="passwordError"
                            @setData="setPassword"
                            width="100%"
                            height="46px"
                            isWhite
                            isPassword>
                        </HeaderlessInput>
                        <span
                            v-html="infoImage"
                            title="Use 6 or more characters with a mix of letters and numbers"></span>
                    </div>
                    <div class="register-input">
                        <HeaderlessInput
                            class="full-input"
                            label="Repeat Password"
                            placeholder="Repeat Password"
                            :error="repeatedPasswordError"
                            @setData="setRepeatedPassword"
                            width="100%"
                            height="46px"
                            isPassword
                            isWhite >
                        </HeaderlessInput>
                        <span v-html="infoImage"></span>
                    </div>
                    <div class="register-area__submit-container">
                        <div class="register-area__submit-container__terms-area">
                            <label class="container">
                                <input type="checkbox" v-model="isTermsAccepted">
                                <span v-bind:class="[isTermsAcceptedError ? 'checkmark error': 'checkmark']"></span>
                            </label>
                            <h2>I agree to the <a>Terms & Conditions</a></h2>
                        </div>
                        <div id="createAccountButton" class="register-area__submit-container__create-button" v-on:click.prevent="onCreateClick">
                            <p>Create Account</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        <RegistrationSuccessPopup />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
import RegistrationSuccessPopup from '@/components/common/RegistrationSuccessPopup.vue';
import { validateEmail, validatePassword } from '@/utils/validation';
import ROUTES from '@/utils/constants/routerConstants';
import { LOADING_CLASSES } from '@/utils/constants/classConstants';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
import { createUserRequest } from '@/api/users';

@Component(
    {
        data: function () {
            return {
                fullName: '',
                fullNameError: '',
                shortName: '',
                email: '',
                emailError: '',
                password: '',
                passwordError: '',
                repeatedPassword: '',
                repeatedPasswordError: '',
                isTermsAccepted: false,
                isTermsAcceptedError: false,
                secret: '',
                loadingClassName: LOADING_CLASSES.LOADING_OVERLAY,
            };
        },
        methods: {
            setEmail: function (value: string): void {
                this.$data.email = value;
                this.$data.emailError = '';
            },
            setFullName: function (value: string): void {
                this.$data.fullName = value;
                this.$data.fullNameError = '';
            },
            setShortName: function (value: string): void {
                this.$data.shortName = value;
            },
            setPassword: function (value: string): void {
                this.$data.password = value;
                this.$data.passwordError = '';
            },
            setRepeatedPassword: function (value: string): void {
                this.$data.repeatedPassword = value;
                this.$data.repeatedPasswordError = '';
            },
            validateFields: function (): boolean {
                let isNoErrors = true;
                if (!this.$data.fullName.trim()) {
                    this.$data.fullNameError = 'Invalid Name';
                    isNoErrors = false;
                }

                if (!validateEmail(this.$data.email.trim())) {
                    this.$data.emailError = 'Invalid Email';
                    isNoErrors = false;
                }

                if (!validatePassword(this.$data.password)) {
                    this.$data.passwordError = 'Invalid Password';
                    isNoErrors = false;
                }

                if (this.$data.repeatedPassword !== this.$data.password) {
                    this.$data.repeatedPasswordError = 'Password doesn\'t match';
                    isNoErrors = false;
                }

                if (!this.$data.isTermsAccepted) {
                    this.$data.isTermsAcceptedError = true;
                    isNoErrors = false;
                }

                return isNoErrors;
            },
            createUser: async function(): Promise<any> {
                let user = {
                    email: this.$data.email.trim(),
                    fullName: this.$data.fullName.trim(),
                    shortName: this.$data.shortName.trim(),
                };

                let response = await createUserRequest(user, this.$data.password, this.$data.secret);
                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);
                    this.$data.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY;

                    return;
                }

                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_REGISTRATION_POPUP);
            },
            onCreateClick: function (): any {
                let self = this as any;

                if (!self.validateFields()) return;

                this.$data.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY_ACTIVE;

                self.createUser();
            },
            onLogoClick: function (): void {
                location.reload();
            },
            onLoginClick: function (): void {
                this.$router.push(ROUTES.LOGIN.path);
            },
        },
        computed: {
            infoImage: function() {

                return EMPTY_STATE_IMAGES.INFO;
            },
        },
        components: {
            HeaderlessInput,
            RegistrationSuccessPopup
        },
        mounted(): void {
            if (this.$route.query.token) {
                this.$data.secret = this.$route.query.token.toString();
            }
        }
    })

export default class Register extends Vue {
}
</script>


<style scoped lang="scss">
    body {
        padding: 0 !important;
        margin: 0 !important;
    }

    .register-container {
        position: fixed;
        width: 100%;
        height: 100%;
        left: 0;
        top: 0;
        z-index: 10;
        background-size: contain;
        display: flex;
        justify-content: flex-start;
        flex-direction: column;
        align-items: flex-start;
        padding: 60px 0px 0px 104px;
        background-image: url("../../static/images/Background.png");
        background-repeat: no-repeat;
        background-size: auto;

        .register-input {
            position: relative;
            width: 100%;

            span {
                position: absolute;
                top: 66px;
                right: 43px;
            }
        }

        &__wrapper {
            min-width: 50%;
            height: 86vh;
        }

        &__header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            flex-direction: row;
            width: 100%;
        }

        &__logo {
            cursor: pointer;
            width: 139px;
            height: 62px;
        }

        &__register-button {
            display: flex;
            align-items: center;
            justify-content: center;
            background-color: transparent;
            border-radius: 6px;
            border: 1px solid white;
            cursor: pointer;
            width: 160px;
            height: 48px;

            p {
                font-family: 'font_bold';
                font-size: 14px;
                line-height: 19px;
                margin-block-start: 0;
                margin-block-end: 0;
                color: white;
            }

            &:hover {
                background-color: white;

                p {
                    color: #2683FF;
                }
            }
        }
    }

    .register-area-wrapper {
        width: 100%;
        height: 100%;
        display: flex;
        align-items: flex-start;
        justify-content: flex-end;
        margin-top: 50px;
    }

    .register-area {
        background-color: transparent;
        width: 620px;
        border-radius: 6px;
        display: flex;
        justify-content: center;
        flex-direction: column;
        align-items: flex-start;

        &__title-container {
            height: 48px;
            display: flex;
            justify-content: space-between;
            align-items: flex-end;
            flex-direction: row;
            margin-bottom: 20px;
            width: 100%;

            h1 {
                font-family: 'font_bold';
                font-size: 22px;
                color: white;
                line-height: 27px;
                margin-block-start: 0;
                margin-block-end: 0;
            }

            p {
                font-family: 'font_regular';
                font-size: 16px;
                color: white;
                line-height: 21px;
                margin-block-start: 0;
                margin-block-end: 0;

                b {
                    font-family: 'font_bold';
                    margin-left: 7px;
                }
            }
        }

        &__submit-container {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            width: 100%;
            margin-top: 20px;

            &__terms-area {
                display: flex;
                flex-direction: row;
                justify-content: center;
                align-items: center;

                &__checkbox {
                    align-self: center;
                }

                h2 {
                    font-family: 'font_regular';
                    font-size: 14px;
                    line-height: 20px;
                    margin-top: 14px;
                    margin-left: 10px;
                    color: white;
                }

                a {
                    color: white;
                    font-family: 'font_bold';

                    &:hover {
                        text-decoration: underline;
                    }
                }

                .container {
                    display: block;
                    position: relative;
                    padding-left: 20px;
                    height: 25px;
                    width: 25px;
                    cursor: pointer;
                    font-size: 22px;
                    -webkit-user-select: none;
                    -moz-user-select: none;
                    -ms-user-select: none;
                    user-select: none;
                    outline: none;
                }

                .container input {
                    position: absolute;
                    opacity: 0;
                    cursor: pointer;
                    height: 0;
                    width: 0;
                }

                .checkmark {
                    position: absolute;
                    top: 0;
                    left: 0;
                    height: 25px;
                    width: 25px;
                    border: 2px solid white;
                    border-radius: 4px;
                }

                .container:hover input ~ .checkmark {
                    background-color: white;
                }

                .container input:checked ~ .checkmark {
                    border: 2px solid white;
                    background-color: transparent;
                }

                .checkmark:after {
                    content: "";
                    position: absolute;
                    display: none;
                }

                .checkmark.error {
                    border-color: red;
                }

                .container input:checked ~ .checkmark:after {
                    display: block;
                }

                .container .checkmark:after {
                    left: 9px;
                    top: 5px;
                    width: 5px;
                    height: 10px;
                    border: solid white;
                    border-width: 0 3px 3px 0;
                    -webkit-transform: rotate(45deg);
                    -ms-transform: rotate(45deg);
                    transform: rotate(45deg);
                }
            }

            &__create-button {
                display: flex;
                align-items: center;
                justify-content: center;
                background-color: #2683FF;
                border-radius: 6px;
                cursor: pointer;
                width: 160px;
                height: 48px;
                box-shadow: 0px 16px 24px #3A54DF;

                p {
                    font-family: 'font_bold';
                    font-size: 14px;
                    line-height: 19px;
                    margin-block-start: 0;
                    margin-block-end: 0;
                    color: white;
                }

                &:hover {
                    box-shadow: none;
                }
            }
        }
    }

    .input-wrap.full-input {
        width: 100%;
    }

    .planet {
        position: absolute;
        bottom: -61px;
        right: -257px;
        z-index: -100;
    }

    .loading-overlay {
        display: flex;
        justify-content: center;
        align-items: center;
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        left: 0;
        height: 100vh;
        z-index: 100;
        background-color: rgba(134, 134, 148, 0.7);
        visibility: hidden;
        opacity: 0;
        -webkit-transition: all 0.5s linear;
        -moz-transition: all 0.5s linear;
        -o-transition: all 0.5s linear;
        transition: all 0.5s linear;

        img {
            z-index: 200;
        }
    }

    .loading-overlay.active {
        visibility: visible;
        opacity: 1;
    }

    @media screen and (max-height: 840px) {
        .register-container {
            overflow: hidden;

            &__wrapper {
                height: 770px;
                overflow-y: scroll;
                overflow-x: hidden;
                -ms-overflow-style: none;
                overflow: -moz-scrollbars-none;

                &::-webkit-scrollbar {
                    width: 0 !important;
                    display: none;
                }
            }
        }
    }

    @media screen and (max-height: 810px) {
        .register-container {
            &__wrapper {
                height: 700px;
            }
        }

        .register-area__submit-container {
            margin-bottom: 25px;
        }
    }
</style>
