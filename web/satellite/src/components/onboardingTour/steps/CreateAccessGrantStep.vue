// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="onboarding-access">
        <h1 class="onboarding-access__title">Create an Access Grant</h1>
        <p class="onboarding-access__sub-title">
            Access Grants are keys that allow access to upload, delete, and view your project’s data.
        </p>
        <div
            class="onboarding-access__content"
            :class="{
                'permissions-margin': isPermissionsStep,
                'passphrase-margin': isPassphraseStep,
                'result-margin': isResultStep,
                'cli-margin': isCLIStep,
            }"
        >
            <ProgressBar v-if="!isCLIStep"/>
            <router-view/>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProgressBar from '@/components/accessGrants/ProgressBar.vue';

import { RouteConfig } from '@/router';

@Component({
    components: {
        ProgressBar,
    },
})
export default class CreateAccessGrantStep extends Vue {
    /**
     * Indicates if current route is access grant permissions step.
     */
    public get isPermissionsStep(): boolean {
        return this.$route.name === RouteConfig.AccessGrantPermissions.name;
    }

    /**
     * Indicates if current route is access grant passphrase step.
     */
    public get isPassphraseStep(): boolean {
        return this.$route.name === RouteConfig.AccessGrantPassphrase.name;
    }

    /**
     * Indicates if current route is access grant CLI step.
     */
    public get isCLIStep(): boolean {
        return this.$route.name === RouteConfig.AccessGrantCLI.name;
    }

    /**
     * Indicates if current route is access grant result step.
     */
    public get isResultStep(): boolean {
        return this.$route.name === RouteConfig.AccessGrantResult.name;
    }
}
</script>

<style scoped lang="scss">
    .onboarding-access {
        font-family: 'font_regular', sans-serif;
        font-style: normal;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
            text-align: center;
            color: #1b2533;
            margin: 0 0 15px 0;
        }

        &__sub-title {
            font-size: 16px;
            line-height: 21px;
            color: #000;
            margin: 0 0 50px 0;
        }

        &__content {
            display: flex;
            align-items: center;
            justify-content: center;
            margin-left: -205px;
        }
    }

    .permissions-margin {
        margin-left: -210px;
    }

    .passphrase-margin {
        margin-left: -190px;
    }

    .result-margin {
        margin-left: -195px;
    }

    .cli-margin {
        margin-left: 0;
    }
</style>
