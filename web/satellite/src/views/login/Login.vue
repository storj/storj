// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./login.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import Button from '@/components/common/Button.vue';
import { setToken } from '@/utils/tokenManager';
import ROUTES from '@/utils/constants/routerConstants';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
import { getTokenRequest } from '@/api/users';
import { LOADING_CLASSES } from '@/utils/constants/classConstants';
import { AppState } from '@/utils/constants/appStateEnum';
import { validateEmail, validatePassword } from '../../utils/validation';

@Component({
    data: function () {
        return {
            email: '',
            password: '',
            loadingClassName: LOADING_CLASSES.LOADING_OVERLAY,
            loadingLogoClassName: LOADING_CLASSES.LOADING_LOGO,
            forgotPasswordRouterPath: ROUTES.FORGOT_PASSWORD.path,
            emailError: '',
            passwordError: '',
        };
    },
    methods: {
        onLogoClick: function (): void {
            location.reload();
        },
        setEmail: function (value: string): void {
            this.$data.email = value;
            this.$data.emailError = '';
        },
        setPassword: function (value: string): void {
            this.$data.password = value;
            this.$data.passwordError = '';
        },
        activateLoadingOverlay: function(): void {
            this.$data.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY_ACTIVE;
            this.$data.loadingLogoClassName = LOADING_CLASSES.LOADING_LOGO_ACTIVE;
        },
        onLogin: async function (): Promise<any> {
            let self = this as any;

            if (!self.validateFields()) {
                return;
            }

            let loginResponse = await getTokenRequest(this.$data.email, this.$data.password);
            if (!loginResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, loginResponse.errorMessage);

                return;
            }

            (this as any).activateLoadingOverlay();

            setTimeout(() => {
                setToken(loginResponse.data);
                this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADING);
                this.$router.push(ROUTES.PROJECT_DETAILS.path);
            }, 2000);
        },
        validateFields: function (): boolean {
            let isNoErrors = true;

            if (!validateEmail(this.$data.email.trim())) {
                this.$data.emailError = 'Invalid Email';
                isNoErrors = false;
            }

            if (!validatePassword(this.$data.password)) {
                this.$data.passwordError = 'Invalid Password';
                isNoErrors = false;
            }

            return isNoErrors;
        },
        onSignUpClick: function (): void {
            this.$router.push(ROUTES.REGISTER.path);
        },
    },
    components: {
        HeaderlessInput,
        Button
    }
})

export default class Login extends Vue {
}
</script>

<style src="./login.scss" scoped lang="scss"></style>
