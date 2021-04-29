// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./registerArea.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import AddCouponCodeInput from '@/components/common/AddCouponCodeInput.vue';
import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import PasswordStrength from '@/components/common/PasswordStrength.vue';
import RegistrationSuccess from '@/components/common/RegistrationSuccess.vue';
import SelectInput from '@/components/common/SelectInput.vue';

import AuthIcon from '@/../static/images/AuthImage.svg';
import BottomArrowIcon from '@/../static/images/common/lightBottomArrow.svg';
import SelectedCheckIcon from '@/../static/images/common/selectedCheck.svg';
import LogoIcon from '@/../static/images/dcs-logo.svg';
import InfoIcon from '@/../static/images/info.svg';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { PartneredSatellite } from '@/types/common';
import { User } from '@/types/users';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { LocalData } from '@/utils/localData';
import { Validator } from '@/utils/validation';

@Component({
    components: {
        HeaderlessInput,
        RegistrationSuccess,
        AuthIcon,
        BottomArrowIcon,
        SelectedCheckIcon,
        LogoIcon,
        InfoIcon,
        PasswordStrength,
        AddCouponCodeInput,
        SelectInput,
    },
})
export default class RegisterArea extends Vue {
    private readonly user = new User();

    // tardigrade logic
    private secret: string = '';

    private userId: string = '';
    private isTermsAccepted: boolean = false;
    private password: string = '';
    private repeatedPassword: string = '';

    // Only for beta sats (like US2).
    private areBetaTermsAccepted: boolean = false;
    private areBetaTermsAcceptedError: boolean = false;

    private fullNameError: string = '';
    private emailError: string = '';
    private passwordError: string = '';
    private repeatedPasswordError: string = '';
    private companyNameError: string = '';
    private employeeCountError: string = '';
    private positionError: string = '';
    private isTermsAcceptedError: boolean = false;
    private isLoading: boolean = false;
    private isProfessional: boolean = false;

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    public isPasswordStrengthShown: boolean = false;

    // tardigrade logic
    public isDropdownShown: boolean = false;

    // Employee Count dropdown options
    public employeeCountOptions = ['1-50', '51-1000', '1001+'];
    public optionsShown = false;

    public readonly loginPath: string = RouteConfig.Login.path;

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
     * Lifecycle hook after initial render.
     * Sets up variables from route params.
     */
    public mounted(): void {
        if (this.$route.query.token) {
            this.secret = this.$route.query.token.toString();
        }

        if (this.$route.query.partner) {
            this.user.partner = this.$route.query.partner.toString();
        }
    }

    /**
     * Indicates if registration successful area shown.
     */
    public get isRegistrationSuccessful(): boolean {
        return this.$store.state.appStateModule.appState.isSuccessfulRegistrationShown;
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
     * Indicates if satellite is in beta.
     */
    public get isBetaSatellite(): boolean {
        return this.$store.state.appStateModule.isBetaSatellite;
    }

    /**
     * Indicates if coupon code ui is enabled
     */
    public get couponCodeUIEnabled(): boolean {
        return this.$store.state.appStateModule.couponCodeUIEnabled;
    }

    /**
     * Sets user's company name field from value string.
     */
    public setCompanyName(value: string): void {
        this.user.companyName = value.trim();
        this.companyNameError = '';
    }

    /**
     * Sets user's company size field from value string.
     */
    public setEmployeeCount(value: string): void {
        this.user.employeeCount = value;
        this.employeeCountError = '';
    }

    /**
     * Sets user's position field from value string.
     */
    public setPosition(value: string): void {
        this.user.position = value.trim();
        this.positionError = '';
    }

    /**
     * toggle user account type
     */
    public toggleAccountType(value: boolean): void {
        this.isProfessional = value;
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

        if (!Validator.email(this.user.email.trim())) {
            this.emailError = 'Invalid Email';
            isNoErrors = false;
        }

        if (!Validator.password(this.password)) {
            this.passwordError = 'Invalid Password';
            isNoErrors = false;
        }

        if (this.isProfessional) {

            if (!this.user.companyName.trim()) {
                this.companyNameError = 'No Company Name filled in';
                isNoErrors = false;
            }

            if (!this.user.position.trim()) {
                this.positionError = 'No Position filled in';
                isNoErrors = false;
            }

            if (!this.user.employeeCount.trim()) {
                this.employeeCountError = 'No Company Size filled in';
                isNoErrors = false;
            }

        }

        if (this.repeatedPassword !== this.password) {
            this.repeatedPasswordError = 'Password doesn\'t match';
            isNoErrors = false;
        }

        if (!this.isTermsAccepted) {
            this.isTermsAcceptedError = true;
            isNoErrors = false;
        }

        // only for beta US2 sats.
        if (this.isBetaSatellite && !this.areBetaTermsAccepted) {
            this.areBetaTermsAcceptedError = true;
            isNoErrors = false;
        }

        return isNoErrors;
    }

    /**
     * Creates user and toggles successful registration area visibility.
     */
    private async createUser(): Promise<void> {
        this.user.isProfessional = this.isProfessional;
        try {
            this.userId = await this.auth.register(this.user, this.secret);
            LocalData.setUserId(this.userId);

            await this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_REGISTRATION);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;
        }
    }
}
</script>

<style src="./registerArea.scss" scoped lang="scss"></style>
