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

    @Component(
        {
            data: function () {
                return {
                    loadingClassName: LOADING_CLASSES.LOADING_OVERLAY,
                    email: '',
                    emailError: '',
                };
            },
            components: {
                HeaderlessInput,
            },
            methods: {
                setEmail: function (value: string): void {
                    this.$data.email = value;
                    this.$data.emailError = '';
                },
                onSendConfigurations: async function (): Promise<any> {
                    let self = this as any;

                    if (!self.validateFields()) {
                        return;
                    }

                    let passwordRecoveryResponse = await forgotPasswordRequest(this.$data.email);
                    if (!passwordRecoveryResponse.isSuccess) {
                        this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, passwordRecoveryResponse.errorMessage);

                        return;
                    }

                    this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Please look for instructions at your email');
                },
                onBackToLoginClick: function() {
                    this.$router.push(ROUTES.LOGIN.path);
                },
                onLogoClick: function () {
                   location.reload();
                },
                validateFields: function (): boolean {
                    const isEmailValid = validateEmail(this.$data.email.trim());

                    if (!isEmailValid) {
                        this.$data.emailError = 'Invalid Email';
                    }

                    return isEmailValid;
                }
            }
        })

    export default class ForgotPassword extends Vue {
    }
</script>

<style src="./forgotPassword.scss" scoped lang="scss"></style>
