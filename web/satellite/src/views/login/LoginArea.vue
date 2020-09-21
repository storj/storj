// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./loginArea.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';

import AuthIcon from '@/../static/images/AuthImage.svg';
import LogoIcon from '@/../static/images/Logo.svg';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { AppState } from '@/utils/constants/appStateEnum';
import { Validator } from '@/utils/validation';

@Component({
    components: {
        HeaderlessInput,
        AuthIcon,
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
     * Changes location to register route.
     */
    public onSignUpClick(): void {
        this.$router.push(RouteConfig.Register.path);
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
            this.$segment.track(SegmentEvent.USER_LOGGED_IN, {
                email: this.email,
            });
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

<style src="./loginArea.scss" scoped lang="scss"></style>
