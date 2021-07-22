// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="login-area" @keyup.enter="onLogin">
        <div class="login-area__logo-wrapper">
            <LogoIcon class="logo" @click="onLogoClick"/>
        </div>
        <div class="login-area__content-area">
            <div class="login-area__content-area">
                <div class="login-area__content-area__activation-banner" v-if="isActivatedBannerShown">
                    <p class="login-area__content-area__activation-banner__message"><b>Success!</b> Account Verified.</p>
                </div>
                <div class="login-area__content-area__container">
                    <div class="login-area__content-area__container__title-area">
                        <h1 class="login-area__content-area__container__title-area__title">Sign In</h1>

                        <div class="login-area__expand" @click.stop="toggleDropdown">
                            <span class="login-area__expand__value">{{ satelliteName }}</span>
                            <BottomArrowIcon />
                            <div class="login-area__expand__dropdown" v-if="isDropdownShown" v-click-outside="closeDropdown">
                                <div class="login-area__expand__dropdown__item" @click.stop="closeDropdown">
                                    <SelectedCheckIcon />
                                    <span class="login-area__expand__dropdown__item__name">{{ satelliteName }}</span>
                                </div>
                                <a v-for="sat in partneredSatellites" class="login-area__expand__dropdown__item" :href="sat.address + '/login'">
                                    {{ sat.name }}
                                </a>
                            </div>
                        </div>
                    </div>
                    <div class="login-area__input-wrapper">
                        <HeaderlessInput
                            class="full-input"
                            label="Email Address"
                            placeholder="example@email.com"
                            :error="emailError"
                            @setData="setEmail"
                            height="46px"
                            width="calc(100% - 2px)"
                        />
                    </div>
                    <div class="login-area__input-wrapper">
                        <HeaderlessInput
                            class="full-input"
                            label="Password"
                            placeholder="Password"
                            :error="passwordError"
                            @setData="setPassword"
                            width="calc(100% - 2px)"
                            height="46px"
                            is-password="true"
                        />
                    </div>
                    <p class="login-area__content-area__container__button" @click.prevent="onLogin">Sign In</p>
                </div>
                <div class="login-area__content-area__forgot-container">
                    <p class="login-area__content-area__forgot-container__reset-msg">Forgot your sign in details?<router-link :to="forgotPasswordPath" class="login-area__content-area__forgot-container__link">
                        Reset Password
                    </router-link></p>
                    <router-link :to="registerPath" class="login-area__content-area__forgot-container__link register-link">
                        Need to create an account?
                    </router-link>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';

import AuthIcon from '@/../static/images/AuthImage.svg';
import BottomArrowIcon from '@/../static/images/common/lightBottomArrow.svg';
import SelectedCheckIcon from '@/../static/images/common/selectedCheck.svg';
import LogoIcon from '@/../static/images/dcs-logo.svg';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { PartneredSatellite } from '@/types/common';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';
import { Validator } from '@/utils/validation';

@Component({
    components: {
        HeaderlessInput,
        AuthIcon,
        BottomArrowIcon,
        SelectedCheckIcon,
        LogoIcon,
    },
})
export default class Login extends Vue {
    private email: string = '';
    private password: string = '';
    private authToken: string = '';
    private isLoading: boolean = false;
    private emailError: string = '';
    private passwordError: string = '';

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    public readonly forgotPasswordPath: string = RouteConfig.ForgotPassword.path;
    public isActivatedBannerShown: boolean = false;

    // Tardigrade logic
    public isDropdownShown: boolean = false;

    public readonly registerPath: string = RouteConfig.Register.path;

    /**
     * Lifecycle hook after initial render.
     * Makes activated banner visible on successful account activation.
     */
    public mounted(): void {
        if (this.$route.query.activated === 'true') {
            this.isActivatedBannerShown = true;
        }
    }

    /**
     * Reloads page.
     */
    public onLogoClick(): void {
        location.reload();
    }

    /**
     * Sets email string on change.
     */
    public setEmail(value: string): void {
        this.email = value;
        this.emailError = '';
    }

    /**
     * Sets password string on change.
     */
    public setPassword(value: string): void {
        this.password = value;
        this.passwordError = '';
    }

    /**
     * Name of the current satellite.
     */
    public get satelliteName(): string {
        return this.$store.state.appStateModule.satelliteName;
    }

    /**
     * Information about partnered satellites, including name and signup link.
     */
    public get partneredSatellites(): PartneredSatellite[] {
        return this.$store.state.appStateModule.partneredSatellites;
    }

    /**
     * Toggles satellite selection dropdown visibility (Tardigrade).
     */
    public toggleDropdown(): void {
        this.isDropdownShown = !this.isDropdownShown;
    }

    /**
     * Closes satellite selection dropdown (Tardigrade).
     */
    public closeDropdown(): void {
        this.isDropdownShown = false;
    }

    /**
     * Performs login action.
     * Then changes location to project dashboard page.
     */
    public async onLogin(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;

        if (!this.validateFields()) {
            this.isLoading = false;

            return;
        }

        try {
            this.authToken = await this.auth.token(this.email, this.password);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;

            return;
        }

        await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADING);
        this.isLoading = false;
        await this.$router.push(RouteConfig.ProjectDashboard.path);
    }

    /**
     * Validates email and password input strings.
     */
    private validateFields(): boolean {
        let isNoErrors = true;

        if (!Validator.email(this.email.trim())) {
            this.emailError = 'Invalid Email';
            isNoErrors = false;
        }

        if (!Validator.password(this.password)) {
            this.passwordError = 'Invalid Password';
            isNoErrors = false;
        }

        return isNoErrors;
    }
}
</script>

<style scoped lang="scss">
    .login-area {
        display: flex;
        flex-direction: column;
        font-family: 'font_regular', sans-serif;
        background-color: #f5f6fa;
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        min-height: 100%;
        overflow-y: scroll;

        &__logo-wrapper {
            text-align: center;
            margin-top: 70px;
        }

        &__divider {
            margin: 0 20px;
            height: 22px;
            width: 2px;
            background-color: #acbace;
        }

        &__input-wrapper {
            margin-top: 20px;
        }

        &__expand {
            display: flex;
            align-items: center;
            cursor: pointer;
            position: relative;

            &__value {
                font-size: 16px;
                line-height: 21px;
                color: #acbace;
                margin-right: 10px;
                font-family: 'font_normal', sans-serif;
                font-weight: 700;
            }

            &__dropdown {
                position: absolute;
                top: 35px;
                left: 0;
                background-color: #fff;
                z-index: 1000;
                border: 1px solid #c5cbdb;
                box-shadow: 0 8px 34px rgba(161, 173, 185, 0.41);
                border-radius: 6px;
                min-width: 250px;

                &__item {
                    display: flex;
                    align-items: center;
                    justify-content: flex-start;
                    padding: 12px 25px;
                    font-size: 14px;
                    line-height: 20px;
                    color: #7e8b9c;
                    cursor: pointer;
                    text-decoration: none;

                    &__name {
                        font-family: 'font_bold', sans-serif;
                        margin-left: 15px;
                        font-size: 14px;
                        line-height: 20px;
                        color: #7e8b9c;
                    }

                    &:hover {
                        background-color: #f2f2f6;
                    }
                }
            }
        }

        &__link {
            display: flex;
            justify-content: center;
            align-items: center;
            width: 191px;
            height: 44px;
            border: 2px solid #376fff;
            border-radius: 6px;
            color: #376fff;
            background-color: #fff;
            cursor: pointer;

            &:hover {
                background-color: #376fff;
                color: #fff;
            }
        }

        &__content-area {
            background-color: #f5f6fa;
            padding: 35px 20px 0 20px;
            display: flex;
            flex-direction: column;
            align-items: center;
            height: calc(100% - 55px);
            border-radius: 20px;

            &__activation-banner {
                padding: 20px;
                background-color: rgba(39, 174, 96, 0.1);
                border: 1px solid #27ae60;
                border-radius: 6px;
                width: 570px;
                margin-bottom: 30px;

                &__message {
                    font-size: 16px;
                    line-height: 21px;
                    color: #27ae60;
                    margin: 0;
                }
            }

            &__container {
                display: flex;
                flex-direction: column;
                padding: 60px 80px;
                background-color: #fff;
                min-width: 450px;
                min-height: 345px;
                border-radius: 20px;

                &__title-area {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;

                    &__title {
                        font-size: 24px;
                        line-height: 49px;
                        letter-spacing: -0.100741px;
                        color: #252525;
                        font-family: 'font_normal', sans-serif;
                        font-weight: 800;
                    }

                    &__satellite {
                        font-size: 16px;
                        line-height: 21px;
                        color: #848484;
                    }
                }

                &__button {
                    font-family: 'font_normal', sans-serif;
                    font-weight: 700;
                    margin-top: 40px;
                    display: flex;
                    justify-content: center;
                    align-items: center;
                    background-color: #376fff;
                    border-radius: 50px;
                    color: #fff;
                    cursor: pointer;
                    width: 100%;
                    height: 48px;

                    &:hover {
                        background-color: #0059d0;
                    }
                }
            }

            &__forgot-container {
                width: 100%;
                align-items: center;
                justify-content: center;
                margin-top: 50px;
                display: block;
                text-align: center;

                &__link {
                    font-family: 'font_medium', sans-serif;
                    text-decoration: none;
                    font-size: 14px;
                    line-height: 18px;
                    color: #376fff;
                }

                &__link.register-link {
                    top: 20px;
                    position: relative;
                }

                &__reset-msg {
                    font-size: 14px;
                    line-height: 18px;
                }
            }

            &__footer {
                display: flex;
                justify-content: center;
                align-items: flex-start;
                margin-top: 140px;
                width: 100%;

                &__copyright {
                    font-size: 12px;
                    line-height: 18px;
                    color: #384b65;
                    padding-bottom: 20px;
                }

                &__link {
                    font-size: 12px;
                    line-height: 18px;
                    margin-left: 30px;
                    color: #376fff;
                    text-decoration: none;
                }
            }
        }
    }

    .logo {
        cursor: pointer;
    }

    .input-wrap.full-input {
        width: calc(100% - 2px);
    }

    @media screen and (max-width: 750px) {

        .login-area {

            &__header {
                padding: 10px 20px;
                width: calc(100% - 40px);
            }

            &__content-area {
                width: 90%;
                margin: 0 auto;

                &__container {
                    min-width: 80%;
                }
            }

            &__expand {

                &__dropdown {
                    left: -200px;
                }
            }
        }
    }

    @media screen and (max-width: 414px) {

        .login-area {

            &__logo-wrapper {
                margin-top: 40px;
            }

            &__content-area {
                padding: 30px 20px 0 20px;

                &__container {
                    padding: 20px 25px;
                    min-width: 90%;
                }
            }
        }
    }

    @media screen and (max-width: 375px) {

        .login-area {

            &__content-area {
                padding: 0 20px 100px 20px;

                &__container {
                    background: transparent;
                    min-width: 100%;
                }
            }
        }
    }
</style>
