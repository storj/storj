// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./login.html"></template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
    import Button from '@/components/common/Button.vue';
    import { AuthToken } from '@/utils/authToken';
    import ROUTES from '@/utils/constants/routerConstants';
    import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
    import { getTokenRequest } from '@/api/users';
    import { LOADING_CLASSES } from '@/utils/constants/classConstants';
    import { AppState } from '@/utils/constants/appStateEnum';
    import { validateEmail, validatePassword } from '@/utils/validation';
    import EVENTS from '../../utils/constants/analyticsEventNames';

    @Component({
        components: {
            HeaderlessInput,
            Button
        }
    })
    export default class Login extends Vue {
        public forgotPasswordRouterPath: string = ROUTES.FORGOT_PASSWORD.path;
        private email: string = '';
        private password: string = '';
        private loadingClassName: string = LOADING_CLASSES.LOADING_OVERLAY;
        private loadingLogoClassName: string = LOADING_CLASSES.LOADING_LOGO;
        private emailError: string = '';
        private passwordError: string = '';

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
            this.$router.push(ROUTES.REGISTER.path);
        }

        public async onLogin(): Promise<void> {
            let self = this;
            this.$segment.track(EVENTS.CLICKED_LOGIN);

            if (!self.validateFields()) {
                return;
            }

            let loginResponse = await getTokenRequest(this.email, this.password);
            if (!loginResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, loginResponse.errorMessage);

                return;
            }

            this.activateLoadingOverlay();

            setTimeout(() => {
                AuthToken.set(loginResponse.data);
                this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADING);
                this.$router.push(ROUTES.PROJECT_OVERVIEW.path + '/' + ROUTES.PROJECT_DETAILS.path);
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

<style src="./login.scss" scoped lang="scss"></style>
