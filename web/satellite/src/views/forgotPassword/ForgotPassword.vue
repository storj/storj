// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./forgotPassword.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';

import AuthIcon from '@/../static/images/AuthImage.svg';
import LogoIcon from '@/../static/images/Logo.svg';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { Validator } from '@/utils/validation';

@Component({
    components: {
        HeaderlessInput,
        AuthIcon,
        LogoIcon,
    },
})
export default class ForgotPassword extends Vue {
    private email: string = '';
    private emailError: string = '';

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    // tardigrade logic
    public isDropdownShown: boolean = false;

    /**
     * Checks if page is inside iframe
     */
    public get isInsideIframe(): boolean {
        return window.self !== window.top;
    }

    public setEmail(value: string): void {
        this.email = value;
        this.emailError = '';
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
        const self = this;

        if (!self.validateFields()) {
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

    public onLogoClick(): void {
        location.reload();
    }

    private validateFields(): boolean {
        const isEmailValid = Validator.email(this.email.trim());

        if (!isEmailValid) {
            this.emailError = 'Invalid Email';
        }

        return isEmailValid;
    }
}
</script>

<style src="./forgotPassword.scss" scoped lang="scss"></style>
