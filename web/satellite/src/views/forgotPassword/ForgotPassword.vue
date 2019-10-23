// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./forgotPassword.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';

import AuthIcon from '@/../static/images/AuthImage.svg';
import LogoIcon from '@/../static/images/Logo.svg';

import { AuthApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { LOADING_CLASSES } from '@/utils/constants/classConstants';
import { validateEmail } from '@/utils/validation';

@Component({
    components: {
        HeaderlessInput,
        AuthIcon,
        LogoIcon,
    },
})
export default class ForgotPassword extends Vue {
    public loadingClassName: string = LOADING_CLASSES.LOADING_OVERLAY;
    private email: string = '';
    private emailError: string = '';

    private readonly auth: AuthApi = new AuthApi();

    public setEmail(value: string): void {
        this.email = value;
        this.emailError = '';
    }

    public async onSendConfigurations(): Promise<void> {
        const self = this;

        if (!self.validateFields()) {
            return;
        }

        try {
            await this.auth.forgotPassword(this.email);
            await this.$notify.success('Please look for instructions at your email');
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    public onBackToLoginClick(): void {
        this.$router.push(RouteConfig.Login.path);
    }

    public onLogoClick(): void {
        location.reload();
    }

    private validateFields(): boolean {
        const isEmailValid = validateEmail(this.email.trim());

        if (!isEmailValid) {
            this.emailError = 'Invalid Email';
        }

        return isEmailValid;
    }
}
</script>

<style src="./forgotPassword.scss" scoped lang="scss"></style>
