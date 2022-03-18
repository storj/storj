// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="forgot-area" @keyup.enter="onSendConfigurations">
        <div class="forgot-area__logo-wrapper">
            <LogoIcon class="forgot-area__logo-wrapper__logo" @click="onLogoClick" />
        </div>
        <div class="forgot-area__content-area">
            <div class="forgot-area__content-area__container">
                <div class="forgot-area__content-area__container__title-area">
                    <h1 class="forgot-area__content-area__container__title-area__title">Reset Password</h1>
                    <div class="forgot-area__expand" @click.stop="toggleDropdown">
                        <span class="forgot-area__expand__value">{{ satelliteName }}</span>
                        <BottomArrowIcon />
                        <div v-if="isDropdownShown" v-click-outside="closeDropdown" class="forgot-area__expand__dropdown">
                            <div class="forgot-area__expand__dropdown__item" @click.stop="closeDropdown">
                                <SelectedCheckIcon />
                                <span class="forgot-area__expand__dropdown__item__name">{{ satelliteName }}</span>
                            </div>
                            <a v-for="sat in partneredSatellites" :key="sat.id" class="forgot-area__expand__dropdown__item" :href="sat.address + '/forgot-password'">
                                {{ sat.name }}
                            </a>
                        </div>
                    </div>
                </div>
                <p class="forgot-area__content-area__container__message">If you’ve forgotten your account password, you can reset it here. Make sure you’re signing in to the right satellite.</p>
                <div class="forgot-area__content-area__container__input-wrapper">
                    <HeaderlessInput
                        label="Email Address"
                        placeholder="user@example.com"
                        :error="emailError"
                        @setData="setEmail"
                    />
                </div>
                <p class="forgot-area__content-area__container__button" @click.prevent="onSendConfigurations">Reset Password</p>
            </div>
            <div class="forgot-area__content-area__login-container">
                <router-link :to="loginPath" class="forgot-area__content-area__login-container__link">
                    Back to Login
                </router-link>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';

import BottomArrowIcon from '@/../static/images/common/lightBottomArrow.svg';
import SelectedCheckIcon from '@/../static/images/common/selectedCheck.svg';
import LogoIcon from '@/../static/images/logo.svg';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { PartneredSatellite } from '@/types/common';
import { Validator } from '@/utils/validation';

// @vue/component
@Component({
    components: {
        HeaderlessInput,
        BottomArrowIcon,
        SelectedCheckIcon,
        LogoIcon,
    },
})
export default class ForgotPassword extends Vue {
    private email = '';
    private emailError = '';

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    // tardigrade logic
    public isDropdownShown = false;

    public readonly loginPath: string = RouteConfig.Login.path;

    /**
     * Sets the email field to the given value.
     */
    public setEmail(value: string): void {
        this.email = value.trim();
        this.emailError = '';
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
     * Sends recovery password email.
     */
    public async onSendConfigurations(): Promise<void> {
        if (!this.validateFields()) {
            return;
        }

        try {
            await this.auth.forgotPassword(this.email);
        } catch (error) {
            await this.$notify.error(error.message);

            return;
        }

        await this.$notify.success('Please look for instructions at your email');
    }

    /**
     * Changes location to Login route.
     */
    public onBackToLoginClick(): void {
        this.$router.push(RouteConfig.Login.path);
    }

    /**
     * Reloads the page.
     */
    public onLogoClick(): void {
        location.reload();
    }

    /**
     * Returns whether the email address is properly structured.
     */
    private validateFields(): boolean {
        const isEmailValid = Validator.email(this.email);

        if (!isEmailValid) {
            this.emailError = 'Invalid Email';
        }

        return isEmailValid;
    }
}
</script>

<style scoped lang="scss">
    .forgot-area {
        display: flex;
        flex-direction: column;
        justify-content: flex-start;
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
            margin: 70px 0;

            &__logo {
                cursor: pointer;
            }
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
                font-family: 'font_regular', sans-serif;
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

        &__content-area {
            width: 100%;
            padding: 0 20px;
            margin-bottom: 50px;
            display: flex;
            flex-direction: column;
            align-items: center;
            box-sizing: border-box;

            &__container {
                width: 610px;
                padding: 60px 80px;
                display: flex;
                flex-direction: column;
                background-color: #fff;
                border-radius: 20px;
                box-sizing: border-box;

                &__title-area {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;

                    &__title {
                        font-size: 24px;
                        margin: 10px 0;
                        letter-spacing: -0.100741px;
                        color: #252525;
                        font-family: 'font_bold', sans-serif;
                        font-weight: 800;
                    }
                }

                &__input-wrapper {
                    margin-top: 20px;
                }

                &__button {
                    font-family: 'font_regular', sans-serif;
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

            &__login-container {
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
            }
        }
    }

    @media screen and (max-width: 750px) {

        .forgot-area {

            &__content-area {

                &__container {
                    width: 100%;
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

        .forgot-area {

            &__logo-wrapper {
                margin: 40px;
            }

            &__content-area {
                padding: 0;

                &__container {
                    padding: 60px 60px;
                    border-radius: 0;
                }
            }
        }
    }
</style>
