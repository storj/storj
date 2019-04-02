// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./forgotPassword.html"></template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import HeaderlessInput from '../../components/common/HeaderlessInput.vue';
    import { LOADING_CLASSES } from '@/utils/constants/classConstants';

    @Component(
        {
            data: function () {
                return {
                    loadingClassName: LOADING_CLASSES.LOADING_OVERLAY,
                };
            },
            components: {
                HeaderlessInput,
            },
        })

    export default class ForgotPassword extends Vue {
    }
</script>

<style src="./forgotPassword.scss" scoped lang="scss"></style>
