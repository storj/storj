// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./registerArea.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import PasswordStrength from '@/components/common/PasswordStrength.vue';
import RegistrationSuccess from '@/components/common/RegistrationSuccess.vue';

import AuthIcon from '@/../static/images/AuthImage.svg';
import InfoIcon from '@/../static/images/info.svg';
import LogoIcon from '@/../static/images/Logo.svg';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { User } from '@/types/users';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { LocalData } from '@/utils/localData';
import { MetaUtils } from '@/utils/meta';
import { validateEmail, validatePassword } from '@/utils/validation';

@Component({
    components: {
        HeaderlessInput,
        RegistrationSuccess,
        AuthIcon,
        LogoIcon,
        InfoIcon,
        PasswordStrength,
    },
})
export default class RegisterArea extends Vue {
    private readonly user = new User();

    // tardigrade logic
    private secret: string = '';
    private referralToken: string = '';
    private refUserId: string = '';

    private userId: string = '';
    private isTermsAccepted: boolean = false;
    private password: string = '';
    private repeatedPassword: string = '';

    private fullNameError: string = '';
    private emailError: string = '';
    private passwordError: string = '';
    private repeatedPasswordError: string = '';
    private isTermsAcceptedError: boolean = false;
    private isLoading: boolean = false;

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    public isPasswordStrengthShown: boolean = false;

    /**
     * Lifecycle hook after initial render.
     * Sets up variables from route params.
     */
    public async mounted(): Promise<void> {
        if (this.$route.query.token) {
            this.secret = this.$route.query.token.toString();
        }

        if (this.$route.query.referralToken) {
            this.referralToken = this.$route.query.referralToken.toString();
        }

        const { ids = '' } = this.$route.params;
        let decoded = '';
        try {
            decoded = atob(ids);
        } catch (error) {
            await this.$notify.error('Invalid Referral URL');

            return;
        }
        const referralIds = ids ? JSON.parse(decoded) : undefined;
        if (referralIds) {
            this.user.partnerId = referralIds.partnerId;
            this.refUserId = referralIds.userId;
        }
    }

    /**
     * Lifecycle hook on component destroy.
     * Sets view to default state.
     */
    public beforeDestroy(): void {
        if (this.isRegistrationSuccessful) {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_REGISTRATION);
        }
    }

    /**
     * Indicates if registration successful area shown.
     */
    public get isRegistrationSuccessful(): boolean {
        return this.$store.state.appStateModule.appState.isSuccessfulRegistrationShown;
    }

    /**
     * Checks if page is inside iframe.
     */
    public get isInsideIframe(): boolean {
        return window.self !== window.top;
    }

    /**
     * Makes password strength container visible.
     */
    public showPasswordStrength(): void {
        this.isPasswordStrengthShown = true;
    }

    /**
     * Hides password strength container.
     */
    public hidePasswordStrength(): void {
        this.isPasswordStrengthShown = false;
    }

    /**
     * Validates input fields and proceeds user creation.
     */
    public async onCreateClick(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;

        if (!this.validateFields()) {
            this.isLoading = false;

            return;
        }

        await this.createUser();

        this.isLoading = false;
    }

    /**
     * Reloads page.
     */
    public onLogoClick(): void {
        location.reload();
    }

    /**
     * Changes location to login route.
     */
    public onLoginClick(): void {
        this.$router.push(RouteConfig.Login.path);
    }

    /**
     * Sets user's email field from value string.
     */
    public setEmail(value: string): void {
        this.user.email = value.trim();
        this.emailError = '';
    }

    /**
     * Sets user's full name field from value string.
     */
    public setFullName(value: string): void {
        this.user.fullName = value.trim();
        this.fullNameError = '';
    }

    /**
     * Sets user's password field from value string.
     */
    public setPassword(value: string): void {
        this.user.password = value.trim();
        this.password = value;
        this.passwordError = '';
    }

    /**
     * Sets user's repeat password field from value string.
     */
    public setRepeatedPassword(value: string): void {
        this.repeatedPassword = value;
        this.repeatedPasswordError = '';
    }

    /**
     * Validates input values to satisfy expected rules.
     */
    private validateFields(): boolean {
        let isNoErrors = true;

        if (!this.user.fullName.trim()) {
            this.fullNameError = 'Invalid Name';
            isNoErrors = false;
        }

        if (!validateEmail(this.user.email.trim())) {
            this.emailError = 'Invalid Email';
            isNoErrors = false;
        }

        if (!validatePassword(this.password)) {
            this.passwordError = 'Invalid Password';
            isNoErrors = false;
        }

        if (this.repeatedPassword !== this.password) {
            this.repeatedPasswordError = 'Password doesn\'t match';
            isNoErrors = false;
        }

        if (!this.isTermsAccepted) {
            this.isTermsAcceptedError = true;
            isNoErrors = false;
        }

        return isNoErrors;
    }

    /**
     * Creates user and toggles successful registration area visibility.
     */
    private async createUser(): Promise<void> {
        try {
            this.userId = this.referralToken ?
                await this.auth.referralRegister(this.user, this.referralToken) :
                await this.auth.register(this.user, this.secret, this.refUserId);

            LocalData.setUserId(this.userId);

            this.$segment.identify(this.userId, {
                email: this.$store.getters.user.email,
                referralToken: this.referralToken,
            });

            if (this.isInsideIframe) {
                const verificationPageURL: string = MetaUtils.getMetaContent('verification-page-url');
                const satelliteName: string = MetaUtils.getMetaContent('satellite-name');
                const url = new URL(verificationPageURL);

                url.searchParams.append('name', satelliteName);

                window.top.location.href = url.href;

                return;
            }

            await this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_REGISTRATION);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;
        }
    }
}
</script>

<style src="./registerArea.scss" scoped lang="scss"></style>
