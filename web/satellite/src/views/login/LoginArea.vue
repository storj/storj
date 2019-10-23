// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./loginArea.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';

import AuthIcon from '@/../static/images/AuthImage.svg';
import LogoIcon from '@/../static/images/Logo.svg';
import LoadingLogoIcon from '@/../static/images/LogoWhite.svg';

import { AuthApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { AuthToken } from '@/utils/authToken';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';
import { LOADING_CLASSES } from '@/utils/constants/classConstants';
import { validateEmail, validatePassword } from '@/utils/validation';

@Component({
    components: {
        HeaderlessInput,
        AuthIcon,
        LogoIcon,
        LoadingLogoIcon,
    },
})
export default class Login extends Vue {
    private email: string = '';
    private password: string = '';
    private authToken: string = '';
    private isLoading: boolean = false;

    private readonly forgotPasswordPath: string = RouteConfig.ForgotPassword.path;
    private loadingClassName: string = LOADING_CLASSES.LOADING_OVERLAY;
    private loadingLogoClassName: string = LOADING_CLASSES.LOADING_LOGO;
    private emailError: string = '';
    private passwordError: string = '';

    private readonly auth: AuthApi = new AuthApi();

    public onLogoClick(): void {
        location.reload();
    }

    public setEmail(value: string): void {
        this.email = value;
        this.emailError = '';
    }

    public setPassword(value: string): void {
        this.password = value;
        this.passwordError = '';
    }

    public onSignUpClick(): void {
        this.$router.push(RouteConfig.Register.path);
    }

    public async onLogin(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;

        const self = this;

        if (!self.validateFields()) {
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

        this.activateLoadingOverlay();

        setTimeout(() => {
            AuthToken.set(this.authToken);
            this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADING);
            this.isLoading = false;
            this.$router.push(RouteConfig.ProjectOverview.with(RouteConfig.ProjectDetails).path);
        }, 2000);
    }

    private validateFields(): boolean {
        let isNoErrors = true;

        if (!validateEmail(this.email.trim())) {
            this.emailError = 'Invalid Email';
            isNoErrors = false;
        }

        if (!validatePassword(this.password)) {
            this.passwordError = 'Invalid Password';
            isNoErrors = false;
        }

        return isNoErrors;
    }

    private activateLoadingOverlay(): void {
        this.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY_ACTIVE;
        this.loadingLogoClassName = LOADING_CLASSES.LOADING_LOGO_ACTIVE;
    }
}
</script>

<style src="./loginArea.scss" scoped lang="scss"></style>
