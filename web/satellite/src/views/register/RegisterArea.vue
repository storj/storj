// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./registerArea.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import RegistrationSuccessPopup from '@/components/common/RegistrationSuccessPopup.vue';
import VInfo from '@/components/common/VInfo.vue';

import AuthIcon from '@/../static/images/AuthImage.svg';
import InfoIcon from '@/../static/images/info.svg';
import LogoIcon from '@/../static/images/Logo.svg';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { User } from '@/types/users';
import { setUserId } from '@/utils/consoleLocalStorage';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
import { LOADING_CLASSES } from '@/utils/constants/classConstants';
import { validateEmail, validatePassword } from '@/utils/validation';

@Component({
    components: {
        HeaderlessInput,
        RegistrationSuccessPopup,
        VInfo,
        AuthIcon,
        LogoIcon,
        InfoIcon,
    },
})
export default class RegisterArea extends Vue {
    private readonly user = new User();

    // tardigrade logic
    private secret: string = '';
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

    private loadingClassName: string = LOADING_CLASSES.LOADING_OVERLAY;

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    mounted(): void {
        if (this.$route.query.token) {
            this.secret = this.$route.query.token.toString();
        }

        const { ids = '' } = this.$route.params;
        let decoded = '';
        try {
            decoded = atob(ids);
        } catch (error) {
            this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Invalid Referral URL');
            this.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY;

            return;
        }
        const referralIds = ids ? JSON.parse(decoded) : undefined;
        if (referralIds) {
            this.user.partnerId = referralIds.partnerId;
            this.refUserId = referralIds.userId;
        }
    }

    public async onCreateClick(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;

        if (!this.validateFields()) {
            this.isLoading = false;

            return;
        }

        this.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY_ACTIVE;

        await this.createUser();

        this.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY;

        this.isLoading = false;
    }
    public onLogoClick(): void {
        location.reload();
    }
    public onLoginClick(): void {
        this.$router.push(RouteConfig.Login.path);
    }
    public setEmail(value: string): void {
        this.user.email = value.trim();
        this.emailError = '';
    }
    public setFullName(value: string): void {
        this.user.fullName = value.trim();
        this.fullNameError = '';
    }
    public setShortName(value: string): void {
        this.user.shortName = value.trim();
    }
    public setPassword(value: string): void {
        this.user.password = value.trim();
        this.password = value;
        this.passwordError = '';
    }
    public setRepeatedPassword(value: string): void {
        this.repeatedPassword = value;
        this.repeatedPasswordError = '';
    }

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

    private async createUser(): Promise<void> {
        try {
            this.userId = await this.auth.register(this.user, this.secret, this.refUserId);

            setUserId(this.userId);

            // TODO: improve it
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_REGISTRATION_POPUP);
            const registrationSuccessPopupRef = this.$refs['register_success_popup'];

            if (registrationSuccessPopupRef) {
                (registrationSuccessPopupRef as any).startResendEmailCountdown();
            }
        } catch (error) {
            this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, error.message);
            this.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY;
            this.isLoading = false;
        }
    }
}
</script>

<style src="./registerArea.scss" scoped lang="scss"></style>
