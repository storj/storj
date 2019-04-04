// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./forgotPassword.html"></template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import HeaderlessInput from '../../components/common/HeaderlessInput.vue';
    import { LOADING_CLASSES } from '@/utils/constants/classConstants';
    import { forgotPasswordRequest } from '@/api/users';

    @Component(
        {
            data: function () {
                return {
                    loadingClassName: LOADING_CLASSES.LOADING_OVERLAY,
                    email: '',
                };
            },
            components: {
                HeaderlessInput,
            },
            methods: {
                setEmail: function (value: string): void {
                    this.$data.email = value;
                },
                onSendConfigurations: async function (): Promise<any> {
                    if (!this.$data.email) {
                        return;
                    }

                    let passwordRecoveryResponse = await forgotPasswordRequest(this.$data.email);
                    if(!passwordRecoveryResponse.isSuccess) {

                    }
                },
            }
        })

    export default class ForgotPassword extends Vue {
    }
</script>

<style src="./forgotPassword.scss" scoped lang="scss"></style>
