// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./login.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import HeaderlessInput from '../../components/common/HeaderlessInput.vue';
import Button from '../../components/common/Button.vue';
import { setToken } from '../../utils/tokenManager';
import ROUTES from '../../utils/constants/routerConstants';
import { NOTIFICATION_ACTIONS } from '../../utils/constants/actionNames';
import { getTokenRequest } from '../../api/users';
import { LOADING_CLASSES } from '../../utils/constants/classConstants';

@Component({
    data: function () {

        return {
            email: '',
            password: '',
            loadingClassName: LOADING_CLASSES.LOADING_OVERLAY,
        };
    },
    methods: {
        onLogoClick: function (): void {
            location.reload();
        },
        setEmail: function (value: string): void {
            this.$data.email = value;
        },
        setPassword: function (value: string): void {
            this.$data.password = value;
        },
        activateLoadingOverlay: function(): void {
            this.$data.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY_ACTIVE;
            setTimeout(() => {
                this.$data.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY;
            }, 2000);
        },
        onLogin: async function (): Promise<any> {
            if (!this.$data.email || !this.$data.password) {
                return;
            }

            (this as any).activateLoadingOverlay();

            let loginResponse = await getTokenRequest(this.$data.email, this.$data.password);
            if (!loginResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, loginResponse.errorMessage);

                return;
            }

            setToken(loginResponse.data);
            this.$router.push(ROUTES.DASHBOARD.path);
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
