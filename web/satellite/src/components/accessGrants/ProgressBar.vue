// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="progress-bar">
        <div class="progress-bar__item">
            <div class="progress-bar__item__line" :class="{ blue: isNameStep }" />
            <p class="progress-bar__item__label" :class="{ checked: isNameStep }">Name Access</p>
        </div>
        <div class="progress-bar__item">
            <div class="progress-bar__item__line" :class="{ blue: isPermissionsStep }" />
            <p class="progress-bar__item__label" :class="{ checked: isPermissionsStep }">Permissions</p>
        </div>
        <div class="progress-bar__item">
            <div class="progress-bar__item__line" :class="{ blue: isPassphraseStep }" />
            <p class="progress-bar__item__label" :class="{ checked: isPassphraseStep }">Passphrase</p>
        </div>
        <div class="progress-bar__item">
            <div class="progress-bar__item__line" :class="{ blue: isResultStep }" />
            <p class="progress-bar__item__label" :class="{ checked: isResultStep }">Access Grant</p>
        </div>
        <div v-if="isGatewayStep" class="progress-bar__item">
            <div class="progress-bar__item__line" :class="{ blue: isGatewayStep }" />
            <p class="progress-bar__item__label" :class="{ checked: isGatewayStep }">S3 Gateway</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';

// @vue/component
@Component
export default class ProgressBar extends Vue {
    /**
     * Indicates if current route is on name step.
     */
    public get isNameStep(): boolean {
        return this.$route.name === RouteConfig.NameStep.name;
    }

    /**
     * Indicates if current route is on permissions step.
     */
    public get isPermissionsStep(): boolean {
        return this.$route.name === RouteConfig.PermissionsStep.name;
    }

    /**
     * Indicates if current route is on passphrase step.
     */
    public get isPassphraseStep(): boolean {
        return this.$route.name === RouteConfig.CreatePassphraseStep.name || this.$route.name === RouteConfig.EnterPassphraseStep.name;
    }

    /**
     * Indicates if current route is on result step.
     */
    public get isResultStep(): boolean {
        return this.$route.name === RouteConfig.ResultStep.name;
    }

    /**
     * Indicates if current route is on gateway step.
     */
    public get isGatewayStep(): boolean {
        return this.$route.name === RouteConfig.GatewayStep.name;
    }
}
</script>

<style scoped lang="scss">
    .progress-bar {
        padding: 55px;
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        justify-content: flex-start;
        background: #f5f6fa;
        border-radius: 6px 0 0 6px;

        &__item {
            display: flex;
            align-items: center;

            &__line {
                width: 7px;
                height: 75px;
                border-radius: 40px;
                background: #dcdde1;
                margin-right: 20px;
            }

            &__label {
                font-family: 'font_regular', sans-serif;
                font-style: normal;
                font-size: 10px;
                line-height: 15px;
                color: rgb(0 0 0 / 40%);
                margin: 0;
                white-space: nowrap;
            }
        }
    }

    .checked {
        font-family: 'font_medium', sans-serif;
        color: #000;
    }

    .blue {
        background: #0068dc;
    }
</style>
