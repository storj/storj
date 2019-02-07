// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="register" v-on:keyup.enter="onCreateClick">
        <div class="register-area">
            <div class="register-area__scrollable">
                <div class="register-area__scrollable__navLabel">
                    <router-link to="/" exact>
                        <svg class="register-area__scrollable__navLabel__back-image" width="19" height="19"
                             viewBox="0 0 19 19" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path fill-rule="evenodd" clip-rule="evenodd"
                                  d="M10.5607 0.43934C11.1464 1.02513 11.1464 1.97487 10.5607 2.56066L5.12132 8H17.5C18.3284 8 19 8.67157 19 9.5C19 10.3284 18.3284 11 17.5 11H5.12132L10.5607 16.4393C11.1464 17.0251 11.1464 17.9749 10.5607 18.5607C9.97487 19.1464 9.02513 19.1464 8.43934 18.5607L0.43934 10.5607C-0.146447 9.97487 -0.146447 9.02513 0.43934 8.43934L8.43934 0.43934C9.02513 -0.146447 9.97487 -0.146447 10.5607 0.43934Z"
                                  fill="#384B65"/>
                        </svg>
                    </router-link>
                    <h1>Sign Up</h1>
                </div>
                <div class="register-area__scrollable__form-area">
                    <HeaderedInput
                            class="full-input"
                            label="First name"
                            placeholder="Enter First Name"
                            :error="firstNameError"
                            @setData="setFirstName">
                    </HeaderedInput>
                    <HeaderedInput
                            class="full-input"
                            label="Last Name"
                            placeholder="Enter Last Name"
                            @setData="setLastName">
                    </HeaderedInput>
                    <HeaderedInput
                            class="full-input"
                            label="Email"
                            placeholder="Enter Email"
                            :error="emailError"
                            @setData="setEmail">
                    </HeaderedInput>
                    <HeaderedInput
                            class="full-input"
                            label="Password"
                            placeholder="Enter Password"
                            :error="passwordError"
                            @setData="setPassword"
                            isPassword>
                    </HeaderedInput>
                    <HeaderedInput
                            class="full-input"
                            label="Repeat Password"
                            placeholder="Repeat Password"
                            :error="repeatedPasswordError"
                            @setData="setRepeatedPassword"
                            isPassword>
                    </HeaderedInput>
                    <!-- end of optional area -->
                    <div class="register-area__scrollable__form-area__terms-area">
                        <Checkbox
                                class="register-area__scrollable__form-area__terms-area__checkbox"
                                @setData="setTermsAccepted"
                                :isCheckboxError="isTermsAcceptedError"/>
                        <h2>I agree to the Storj Bridge Hosting <a>Terms & Conditions</a></h2>
                    </div>
                    <Button id="createAccountButton" class="register-area__scrollable__form-area__create-button" label="Create Account"
                            width="100%" height="48px" :onPress="onCreateClick"/>
                </div>
            </div>

        </div>

        <img class="layout-image" src="../../static/images/register/RegisterImage.svg"/>
        <RegistrationSuccessPopup />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import HeaderedInput from '@/components/common/HeaderedInput.vue';
import Checkbox from '@/components/common/Checkbox.vue';
import Button from '@/components/common/Button.vue';
import RegistrationSuccessPopup from '@/components/common/RegistrationSuccessPopup.vue';
import { validateEmail, validatePassword } from '@/utils/validation';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
import { createUserRequest } from '@/api/users';

@Component(
    {
        methods: {
            setEmail: function (value: string) {
                this.$data.email = value;
                this.$data.emailError = '';
            },
            setFirstName: function (value: string) {
                this.$data.firstName = value;
                this.$data.firstNameError = '';
            },
            setLastName: function (value: string) {
                this.$data.lastName = value;
            },
            setPassword: function (value: string) {
                this.$data.password = value;
                this.$data.passwordError = '';
            },
            setRepeatedPassword: function (value: string) {
                this.$data.repeatedPassword = value;
                this.$data.repeatedPasswordError = '';
            },
            setTermsAccepted: function (value: boolean) {
                this.$data.isTermsAccepted = value;
                this.$data.isTermsAcceptedError = false;
            },
            onCreateClick: async function () {
                let hasError = false;
                const firstName = this.$data.firstName.trim();
                const email = this.$data.email.trim();
                const lastName = this.$data.lastName.trim();

                if (!firstName) {
                    this.$data.firstNameError = 'Invalid First Name';
                    hasError = true;
                }

                if (!validateEmail(email)) {
                    this.$data.emailError = 'Invalid Email';
                    hasError = true;
                }

                if (!validatePassword(this.$data.password)) {
                    this.$data.passwordError = 'Invalid Password';
                    hasError = true;
                }

                if (this.$data.repeatedPassword !== this.$data.password) {
                    this.$data.repeatedPasswordError = 'Password doesn\'t match';
                    hasError = true;
                }

                if (!this.$data.isTermsAccepted) {
                    this.$data.isTermsAcceptedError = true;
                    hasError = true;
                }

                if (hasError) return;

                let user = {
                    email,
                    firstName,
                    lastName,
                };

                let response = await createUserRequest(user, this.$data.password);
                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);

                    return;
                }

                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_REGISTRATION_POPUP);
            }
        },
        data: function (): RegisterData {

            return {
                firstName: '',
                firstNameError: '',
                lastName: '',
                email: '',
                emailError: '',
                password: '',
                passwordError: '',
                repeatedPassword: '',
                repeatedPasswordError: '',
                isTermsAccepted: false,
                isTermsAcceptedError: false,
            };
        },
        computed: {},
        components: {
            HeaderedInput,
            Checkbox,
            Button,
            RegistrationSuccessPopup
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

    .register {
        position: fixed;
        background-color: #fff;
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        align-items: center;
        height: 100vh;
        width: 100%;
        overflow: hidden;
    }

    .register-area {
        background-color: white;
        width: 100%;
        max-height: 100vh;

        &__scrollable {
            height: 100vh;
            display: flex;
            flex-direction: column;
            justify-content: flex-start;

            &__navLabel {
                display: flex;
                align-items: center;
                flex-direction: row;
                justify-content: flex-start;
                align-self: center;
                width: 68%;
                margin-top: 70px;
                h1 {
                    color: #384B65;
                    margin-left: 24px;
                    font-family: 'montserrat_bold'
                }

                &__back-image {
                    width: 21px;
                    height: 21px;

                    &:hover path {
                        fill: #2683FF !important;
                    }
                }
            }

            &__form-area {
                margin-top: 50px;
                align-self: center;
                width: 35vw;

                &__company-area {
                    display: flex;
                    flex-direction: row;
                    justify-content: space-between;
                    align-items: center;
                    width: 100%;
                    margin-top: 32px;
                    h2 {
                        font-family: 'montserrat_bold';
                        font-size: 20px;
                        line-height: 27px;
                        margin-right: 11px;
                    }
                ;
                    &__details-area {
                        cursor: pointer;
                        display: flex;
                        flex-direction: row;
                        justify-content: center;
                        align-items: center;

                        &__text {
                            font-size: 16px;
                            line-height: 23px;
                        }

                        &__expander-area {
                            display: flex;
                            align-items: center;
                            justify-content: center;
                            width: 28px;
                            height: 28px;
                            border-radius: 4px;

                            &:hover {
                                background-color: #E2ECF7;
                            }
                        }
                    }
                ;
                }

                &__terms-area {
                    display: flex;
                    flex-direction: row;
                    justify-content: flex-start;
                    margin-top: 20px;

                    &__checkbox {
                        align-self: center;
                    }
                ;

                    h2 {
                        font-family: 'montserrat_regular';
                        font-size: 14px;
                        line-height: 20px;
                        margin-top: 30px;
                        margin-left: 10px;
                    }
                ;
                    a {
                        color: #2683FF;
                        font-family: 'montserrat_bold';

                        &:hover {
                            text-decoration: underline;
                        }
                    }
                }

                &__create-button {
                    margin-top: 30px;
                    margin-bottom: 100px;
                }
            }
        }
    }

    .input-container.full-input {
        width: 100%;
    }

    .layout-image {
        background-color: #2683FF;
        display: block;
        height: 100vh;
    }

    a {
        cursor: pointer;
    }

    #optional-area {
        height: auto;
        visibility: visible;
        opacity: 1;
        transition: 0.5s;
    }

    #optional-area.optional-area--active {
        animation: mymove 0.5s ease-in-out;
    }

    #optional-area.optional-area {
        height: 0px;
        visibility: hidden;
        position: absolute;
        animation: mymoveout 0.5s ease-in-out;
    }

    @keyframes mymove {
        from {
            height: 0px;
            visibility: hidden;
            opacity: 0;
        }
        to {
            height: 100%;
            visibility: visible;
            opacity: 1;
        }
    }

    @keyframes mymoveout {
        from {
            height: 100%;
            visibility: visible;
            opacity: 1;
        }
        to {
            height: 0px;
            visibility: hidden;
            opacity: 0;
        }
    }

    @media screen and (max-width: 1440px) {
        .register-area {
            background-color: white;
            width: 100%;
            max-height: 100vh;

            &__scrollable {
                 width: 60vw;
            }
        }
    }

    @media screen and (max-width: 720px), screen and (max-height: 880px) {
        .register {
            flex-direction: column;
        }
        .layout-image {
            position: absolute;
            display: block;
            bottom: 0;
            left: 0;
            width: 100%;
            height: 200px;
        }
        .register-area {
            width: 100vw;
            overflow-y: scroll;
            height: calc(100vh - 200px);

            &__scrollable {
                 width: auto;

                &__form-area {

                    &__create-button {
                         margin-bottom: 50px;
                    }
                }

                &__form-area {
                     width: 75vw;
                }
            }
        }
    }

</style>