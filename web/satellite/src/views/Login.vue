// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="login-container" v-on:keyup.enter="onLogin">
        <img class="planet" src="../../static/images/Mars.png" alt="" >
        <div class="login-container__wrapper">
            <div class="login-container__header">
                <img class="login-container__logo" src="../../static/images/login/Logo.svg" alt="logo" v-on:click="onLogoClick">
                <div class="login-container__register-button" v-on:click.prevent="onSignUpPress">
                    <p>Create Account</p>
                </div>
            </div>
            <div class="login-area-wrapper">
                <div class="login-area">
                    <div class="login-area__title-container">
                        <h1>Login to Storj</h1>
                        <p>Satellite:<b>Mars</b></p>
                    </div>
                    <HeaderlessInput
                            class="login-area__email-input"
                            placeholder="Email"
                            @setData="setEmail"
                            height="46px"
                            width="100%">
                    </HeaderlessInput>
                    <HeaderlessInput
                            class="login-area__password-input"
                            placeholder="Password"
                            @setData="setPassword"
                            width="100%"
                            height="46px"
                            isPassword>
                    </HeaderlessInput>
                    <div class="login-area__submit-area">
                        <router-link to="" class="login-area__navigation-area__nav-link" exact>
                            <h3><strong>Forgot password?</strong></h3>
                        </router-link>
                        <div class="login-area__submit-area__login-button" v-on:click.prevent="onLogin">
                            <p>Login</p>
                        </div>
                    </div>
                    <div class="login-area__info-area">
                        <p class="login-area__info-area__signature">Storj Labs Inc 2019.</p>
                        <a class="login-area__info-area__terms">Terms & Conditions</a>
                        <a class="login-area__info-area__help">Help</a>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import Button from '@/components/common/Button.vue';
import { setToken } from '@/utils/tokenManager';
import ROUTES from '../utils/constants/routerConstants';
import { NOTIFICATION_ACTIONS } from '../utils/constants/actionNames';
import { getTokenRequest } from '@/api/users';

@Component({
    data: function () {

        return {
            email: '',
            password: '',
            token: ''
        };
    },
    methods: {
        onLogoClick: function (): void {
            location.reload();
        },
        setEmail: function (value: string) {
            this.$data.email = value;
        },
        setPassword: function (value: string) {
            this.$data.password = value;
        },
        onLogin: async function () {
            let loginResponse = await getTokenRequest(this.$data.email, this.$data.password);
            if (!loginResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, loginResponse.errorMessage);

                return;
            }

            setToken(loginResponse.data);
            this.$router.push(ROUTES.DASHBOARD.path);
        }

    },
    components: {
        HeaderlessInput,
        Button
    }
})

export default class Login extends Vue {
}
</script>

<style scoped lang="scss">
    .login-container {
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
                font-family: 'montserrat_bold';
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

    .planet {
        position: absolute;
        top: -161px;
        right: -257px;
        z-index: -100;
    }

    .login-area-wrapper {
        width: 100%;
        height: 100%;
        display: flex;
        align-items: flex-end;
        justify-content: flex-end;
    }

    .login-area {
        background-color: transparent;
        width: 620px;
        border-radius: 6px;
        display: flex;
        justify-content: center;
        flex-direction: column;
        align-items: flex-start;
        padding-bottom: 50px;

        &__title-container {
            height: 48px;
            display: flex;
            justify-content: space-between;
            align-items: flex-end;
            flex-direction: row;
            margin-bottom: 20px;
            width: 100%;

            h1 {
                font-family: 'montserrat_bold';
                font-size: 22px;
                color: white;
                line-height: 27px;
                margin-block-start: 0;
                margin-block-end: 0;
            }

            p {
                font-family: 'montserrat_regular';
                font-size: 16px;
                color: white;
                line-height: 21px;
                margin-block-start: 0;
                margin-block-end: 0;

                b {
                    font-family: 'montserrat_bold';
                    margin-left: 7px;
                }
            }
        }

        &__password-input {
            margin-top: 22px;
        }

        &__submit-area {
            display: flex;
            justify-content: space-between;
            flex-direction: row;
            align-items: center;
            width: 100%;
            margin-top: 22px;

            &__login-button {
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
                    font-family: 'montserrat_bold';
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

        &__info-area {
            width: 100%;
            height: 42px;
            margin-top: 300px;
            display: flex;
            align-items: flex-end;
            justify-content: flex-start;
            flex-direction: row;

            p {
                font-family: 'montserrat_regular';
                font-size: 12px;
                line-height: 18px;
                text-align: center;
                text-decoration: none;
                color: white;
                margin-block-start: 0;
                margin-block-end: 0;
            }

            a {
                font-family: 'montserrat_regular';
                font-size: 15px;
                line-height: 22px;
                text-align: center;
                text-decoration: none;
                color: white;
            }

            &__signature {
                margin-right: 50px;
            }

            &__terms {
                margin-right: 35px;
            }
        }

        &__login-button.container {
            display: block;
            text-align: center;
        }

        &__navigation-area {
            margin-top: 24px;
            width: 100%;
            height: 48px;
            display: flex;
            justify-content: center;
            flex-direction: row;
            align-items: center;

            &__nav-link {
                font-family: 'montserrat_regular';
                font-size: 14px;
                line-height: 20px;
                height: 48px;
                text-align: center;
                padding-left: 15px;
                padding-right: 15px;
                min-width: 140px;
                text-decoration: none;
                color: white;

                .bold {
                    font-family: 'montserrat_medium';
                }
            }
        }
    }
</style>
