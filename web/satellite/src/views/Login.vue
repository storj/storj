// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="login-container" v-on:keyup.enter="onLogin">
        <div class="login-container__wrapper">
            <img class="login-container__logo" src="../../static/images/login/Logo.svg" alt="logo" v-on:click="onLogoClick">
            <div class="login-area">
                <div class="login-area__title-container">
                    <h1>Welcome to Storj</h1>
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
                <Button class="login-area__login-button" label="Login" width="100%" height="48px" :onPress="onLogin"/>
                <!-- start of navigation area -->
                <div class="login-area__navigation-area">
                    <router-link to="/register" class="login-area__navigation-area__nav-link bold" exact><h3>Create
                        account</h3></router-link>
                    <router-link to="" class="login-area__navigation-area__nav-link" exact><h3><strong>Forgot
                        password</strong></h3></router-link>
                </div>
                <!-- end of navigation area -->
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
        background: rgba(51, 51, 51, 0.7);
        z-index: 10;
        background-image: url(../../static/images/login/1920.svg);
        background-repeat: no-repeat;
        background-size: contain;
        display: flex;
        justify-content: flex-start;
        flex-direction: column;
        align-items: flex-start;
        padding: 60px 0px 190px 104px;

        &__logo {
            width: 139px;
            height: 62px;
        }
    }

    .login-area {
        background-color: #fff;
        margin-top: 50px;
        max-width: 500px;
        width: 100%;
        padding: 120px;
        border-radius: 6px;
        display: flex;
        justify-content: center;
        flex-direction: column;
        align-items: flex-start;

        &__title-container {
            height: 48px;
            display: flex;
            justify-content: flex-start;
            align-items: flex-start;
            margin-bottom: 32px;

            h1 {
                font-family: 'montserrat_bold';
                font-size: 32px;
                color: #384B65;
                line-height: 39px;
                margin-block-start: 0;
                margin-block-end: 0;
            }
        }

        &__password-input {
            margin-top: 22px;
        }

        &__login-button {
            margin-top: 22px;
            align-self: center;
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
                color: #2683FF;
                height: 48px;
                text-align: center;
                text-justify: center;
                padding-left: 15px;
                padding-right: 15px;
                min-width: 140px;

                &:hover {
                    text-decoration: underline;
                }

                .bold {
                    font-family: 'montserrat_medium';
                }
            }
        }
    }

    @media screen and (max-width: 1440px) {
        .login-container {
            background-size: auto;
            background-image: url(../../static/images/login/Background.svg);
        }
    }

    @media screen and (max-width: 1280px) {
        .login-container {
            background-image: url(../../static/images/login/1280.svg);
            background-size: auto;
        }
        .login-area {
            padding: 86px;
            max-width: 444px;
        }
    }

    @media screen and (max-width: 1024px) {
        .login-container {
            background-image: url(../../static/images/login/1024.svg);
        }
    }

    @media screen and (max-width: 800px) {
        .login-container {
            padding: 0;
            justify-content: flex-start;
            display: block;
            padding: 77px 50px 0 50px;
            background-image: url(../../static/images/login/800.svg);
            background-position-y: 0px;
            width: auto;
            height: 1450px;
            position: relative;

            &__wrapper {
                margin: 0 auto;
                max-width: 600px;
            }
        }
        .login-area {
            max-width: auto;
            width: auto;
            margin: 0 auto;
            margin-top: 80px;
        }
    }
</style>
