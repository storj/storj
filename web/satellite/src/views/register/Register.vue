// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./register.html"></template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import HeaderlessInput from '../../components/common/HeaderlessInput.vue';
    import RegistrationSuccessPopup from '../../components/common/RegistrationSuccessPopup.vue';
    import { validateEmail, validatePassword } from '../../utils/validation';
    import ROUTES from '../../utils/constants/routerConstants';
    import EVENTS from '../../utils/constants/analyticsEventNames';
    import { LOADING_CLASSES } from '../../utils/constants/classConstants';
    import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS } from '../../utils/constants/actionNames';
    import { createUserRequest } from '../../api/users';
    import { setUserId } from '@/utils/consoleLocalStorage';
    import { User } from '../../types/users';
    import InfoComponent from '../../components/common/InfoComponent.vue';

    @Component({
        components: {
            HeaderlessInput,
            RegistrationSuccessPopup,
            InfoComponent,
        },
    })
    export default class Register extends Vue {
        private fullName: string = '';
        private fullNameError: string = '';
        private shortName: string = '';
        private email: string = '';
        private emailError: string = '';
        private password: string = '';
        private passwordError: string = '';
        private repeatedPassword: string = '';
        private repeatedPasswordError: string = '';
        private isTermsAccepted: boolean = false;
        private isTermsAcceptedError: boolean = false;
        private secret: string = '';
        private partnerId: string = '';
        private refUserId: string = '';
        private loadingClassName: string = LOADING_CLASSES.LOADING_OVERLAY;

        mounted(): void {
            if (this.$route.query.token) {
                this.secret = this.$route.query.token.toString();
            }

            let { ids } = this.$route.params;
            let referralIds = ids ? JSON.parse(atob(ids)) : undefined;
            if (referralIds) {
                this.$data.partnerId = referralIds.partnerId;
                this.$data.refUserId = referralIds.userId;
            }
        }

        public onCreateClick(): void {
            if (!this.validateFields()) {
                return;
            }

            this.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY_ACTIVE;

            this.createUser();

            this.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY;
        }
        public onLogoClick(): void {
            this.$segment.track(EVENTS.CLICKED_LOGO);
            location.reload();
        }
        public onLoginClick(): void {
            this.$segment.track(EVENTS.CLICKED_LOGIN);
            this.$router.push(ROUTES.LOGIN.path);
        }
        public setEmail(value: string): void {
            this.email = value;
            this.emailError = '';
        }
        public setFullName(value: string): void {
            this.fullName = value;
            this.fullNameError = '';
        }
        public setShortName(value: string): void {
            this.shortName = value;
        }
        public setPassword(value: string): void {
            this.password = value;
            this.passwordError = '';
        }
        public setRepeatedPassword(value: string): void {
            this.repeatedPassword = value;
            this.repeatedPasswordError = '';
        }

        private validateFields(): boolean {
            let isNoErrors = true;

            if (!this.fullName.trim()) {
                this.fullNameError = 'Invalid Name';
                isNoErrors = false;
            }

            if (!validateEmail(this.email.trim())) {
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
            let user = new User(this.fullName.trim(), this.shortName.trim(), this.email.trim(), this.partnerId);
            let response = await createUserRequest(user, this.password, this.secret, this.refUserId);
            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);
                this.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY;

                return;
            }
            if (response.data) {
                setUserId(response.data);
            }
            // TODO: improve it
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_REGISTRATION_POPUP);
            if (this.$refs['register_success_popup'] !== null) {
                (this.$refs['register_success_popup'] as any).startResendEmailCountdown();
            }
        }
    }
</script>

<style src="./register.scss" scoped lang="scss"></style>
