// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="activate-area">
        <div class="activate-area__logo-wrapper">
            <LogoIcon class="activate-area__logo-wrapper_logo" @click="onLogoClick" />
        </div>
        <div class="activate-area__content-area">
            <RegistrationSuccess v-if="isRegistrationSuccessShown" :email="email" />
            <div v-else class="activate-area__content-area__container">
                <h1 class="activate-area__content-area__container__title">Activate Account</h1>
                <div class="activate-area__content-area__container__input-wrapper">
                    <VInput
                        label="Email Address"
                        placeholder="user@example.com"
                        :error="emailError"
                        height="46px"
                        width="100%"
                        @setData="setEmail"
                    />
                </div>
                <p class="activate-area__content-area__container__button" @click.prevent="onActivateClick">Activate</p>
            </div>
            <router-link :to="loginPath" class="activate-area__content-area__login-link">
                Back to Login
            </router-link>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { Validator } from '@/utils/validation';
import { MetaUtils } from '@/utils/meta';

import RegistrationSuccess from '@/components/common/RegistrationSuccess.vue';
import VInput from '@/components/common/VInput.vue';

import LogoIcon from '@/../static/images/logo.svg';

// @vue/component
@Component({
    components: {
        LogoIcon,
        VInput,
        RegistrationSuccess,
    },
})
export default class ActivateAccount extends Vue {
    private email = '';
    private emailError = '';
    private isRegistrationSuccessShown = false;

    public readonly loginPath: string = RouteConfig.Login.path;

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    /**
     * onActivateClick validates input fields and requests resending of activation email.
     */
    public async onActivateClick(): Promise<void> {
        if (!Validator.email(this.email)) {
            this.emailError = 'Invalid email';
            return;
        }

        try {
            await this.auth.resendEmail(this.email);
            this.isRegistrationSuccessShown = true;
        } catch (error) {
            this.$notify.error(error.message);
        }
    }

    /**
     * setEmail sets the email property to the given value.
     */
    public setEmail(value: string): void {
        this.email = value.trim();
        this.emailError = '';
    }

    /**
     * Redirects to storj.io homepage.
     */
    public onLogoClick(): void {
        const homepageURL = MetaUtils.getMetaContent('homepage-url');
        window.location.href = homepageURL;
    }
}
</script>

<style lang="scss" scoped>
    .activate-area {
        display: flex;
        flex-direction: column;
        justify-content: flex-start;
        align-items: center;
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
                width: 207px;
                height: 37px;
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

                &__input-wrapper {
                    margin-top: 20px;
                }

                &__title {
                    font-size: 24px;
                    margin: 10px 0;
                    color: #252525;
                    font-family: 'font_bold', sans-serif;
                }

                &__button {
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

            &__login-link {
                font-family: 'font_medium', sans-serif;
                text-decoration: none;
                font-size: 14px;
                color: #376fff;
                margin-top: 50px;
            }
        }
    }

    @media screen and (max-width: 750px) {

        .activate-area {

            &__content-area {

                &__container {
                    width: 100%;
                    padding: 60px;
                }
            }
        }
    }

    @media screen and (max-width: 414px) {

        .activate-area {

            &__logo-wrapper {
                margin: 40px;
            }

            &__content-area {
                padding: 0;

                &__container {
                    padding: 20px;
                    padding-top: 0;
                    background: transparent;
                }
            }
        }
    }
</style>
