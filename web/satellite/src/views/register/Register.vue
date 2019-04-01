// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./register.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import HeaderlessInput from '../../components/common/HeaderlessInput.vue';
import { EMPTY_STATE_IMAGES } from '../../utils/constants/emptyStatesImages';
import RegistrationSuccessPopup from '../../components/common/RegistrationSuccessPopup.vue';
import { validateEmail, validatePassword } from '../../utils/validation';
import ROUTES from '../../utils/constants/routerConstants';
import { LOADING_CLASSES } from '../../utils/constants/classConstants';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS } from '../../utils/constants/actionNames';
import { createUserRequest } from '../../api/users';

@Component(
    {
        data: function () {
            return {
                fullName: '',
                fullNameError: '',
                shortName: '',
                email: '',
                emailError: '',
                password: '',
                passwordError: '',
                repeatedPassword: '',
                repeatedPasswordError: '',
                isTermsAccepted: false,
                isTermsAcceptedError: false,
                secret: '',
                loadingClassName: LOADING_CLASSES.LOADING_OVERLAY,
            };
        },
        methods: {
            setEmail: function (value: string): void {
                this.$data.email = value;
                this.$data.emailError = '';
            },
            setFullName: function (value: string): void {
                this.$data.fullName = value;
                this.$data.fullNameError = '';
            },
            setShortName: function (value: string): void {
                this.$data.shortName = value;
            },
            setPassword: function (value: string): void {
                this.$data.password = value;
                this.$data.passwordError = '';
            },
            setRepeatedPassword: function (value: string): void {
                this.$data.repeatedPassword = value;
                this.$data.repeatedPasswordError = '';
            },
            validateFields: function (): boolean {
                let isNoErrors = true;
                if (!this.$data.fullName.trim()) {
                    this.$data.fullNameError = 'Invalid Name';
                    isNoErrors = false;
                }

                if (!validateEmail(this.$data.email.trim())) {
                    this.$data.emailError = 'Invalid Email';
                    isNoErrors = false;
                }

                if (!validatePassword(this.$data.password)) {
                    this.$data.passwordError = 'Invalid Password';
                    isNoErrors = false;
                }

                if (this.$data.repeatedPassword !== this.$data.password) {
                    this.$data.repeatedPasswordError = 'Password doesn\'t match';
                    isNoErrors = false;
                }

                if (!this.$data.isTermsAccepted) {
                    this.$data.isTermsAcceptedError = true;
                    isNoErrors = false;
                }

                return isNoErrors;
            },
            createUser: async function(): Promise<any> {
                let user = {
                    email: this.$data.email.trim(),
                    fullName: this.$data.fullName.trim(),
                    shortName: this.$data.shortName.trim(),
                };

                let response = await createUserRequest(user, this.$data.password, this.$data.secret);
                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);
                    this.$data.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY;

                    return;
                }

                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_REGISTRATION_POPUP);
            },
            onCreateClick: function (): any {
                let self = this as any;

                if (!self.validateFields()) return;

                this.$data.loadingClassName = LOADING_CLASSES.LOADING_OVERLAY_ACTIVE;

                self.createUser();
            },
            onLogoClick: function (): void {
                location.reload();
            },
            onLoginClick: function (): void {
                this.$router.push(ROUTES.LOGIN.path);
            },
        },
        computed: {
            infoImage: function() {

                return EMPTY_STATE_IMAGES.INFO;
            },
        },
        components: {
            HeaderlessInput,
            RegistrationSuccessPopup
        },
        mounted(): void {
            if (this.$route.query.token) {
                this.$data.secret = this.$route.query.token.toString();
            }
        }
    })

export default class Register extends Vue {
}
</script>

<style src="./register.scss" scoped lang="scss"></style>
