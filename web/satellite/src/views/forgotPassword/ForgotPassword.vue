// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./forgotPassword.html"></template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
    import { LOADING_CLASSES } from '@/utils/constants/classConstants';
    import { forgotPasswordRequest } from '@/api/users';
    import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
    import ROUTES from '@/utils/constants/routerConstants';
    import { validateEmail } from '@/utils/validation';
    import EVENTS from '@/utils/constants/analyticsEventNames';

    @Component({
        components: {
            HeaderlessInput,
        },
    })
    export default class ForgotPassword extends Vue {
        public loadingClassName: string = LOADING_CLASSES.LOADING_OVERLAY;
        private email: string = '';
        private emailError: string = '';

        public setEmail(value: string): void {
            this.email = value;
            this.emailError = '';
        }

        public async onSendConfigurations(): Promise<void> {
            let self = this;

            if (!self.validateFields()) {
                return;
            }

            let passwordRecoveryResponse = await forgotPasswordRequest(this.email);
            if (!passwordRecoveryResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, passwordRecoveryResponse.errorMessage);

                return;
            }

            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Please look for instructions at your email');
        }

        public onBackToLoginClick(): void {
            this.$segment.track(EVENTS.CLICKED_BACK_TO_LOGIN);
            this.$router.push(ROUTES.LOGIN.path);
        }

        public onLogoClick(): void {
            this.$segment.track(EVENTS.CLICKED_LOGO);
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
