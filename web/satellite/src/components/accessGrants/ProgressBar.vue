// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="progress-bar">
        <div class="progress-bar__item">
            <div class="progress-bar__item__circle" :class="{ blue: isNameStep || isPermissionsStep || isPassphraseStep || isResultStep }"/>
            <p class="progress-bar__item__label" :class="{ checked: isNameStep || isPermissionsStep || isPassphraseStep || isResultStep }">Name Access</p>
        </div>
        <div class="progress-bar__progress" :class="{ blue: isPermissionsStep || isPassphraseStep || isResultStep }"/>
        <div class="progress-bar__item">
            <div class="progress-bar__item__circle" :class="{ blue: isPermissionsStep || isPassphraseStep || isResultStep }"/>
            <p class="progress-bar__item__label" :class="{ checked: isPermissionsStep || isPassphraseStep || isResultStep }">Permissions</p>
        </div>
        <div class="progress-bar__progress" :class="{ blue: isPassphraseStep || isResultStep }"/>
        <div class="progress-bar__item">
            <div class="progress-bar__item__circle" :class="{ blue: isPassphraseStep || isResultStep }"/>
            <p class="progress-bar__item__label" :class="{ checked: isPassphraseStep || isResultStep }">Passphrase</p>
        </div>
        <div class="progress-bar__progress" :class="{ blue: isResultStep }"/>
        <div class="progress-bar__item">
            <div class="progress-bar__item__circle" :class="{ blue: isResultStep }"/>
            <p class="progress-bar__item__label" :class="{ checked: isResultStep }">Access Grant</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';

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
}
</script>

<style scoped lang="scss">
    .progress-bar {
        padding: 55px;
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        background: #f5f6fa;
        height: 380px;
        border-radius: 6px 0 0 6px;

        &__item {
            display: flex;
            align-items: center;

            &__circle {
                width: 20px;
                height: 20px;
                border-radius: 10px;
                background: #dcdde1;
                margin-right: 10px;
            }

            &__label {
                font-family: 'font_regular', sans-serif;
                font-style: normal;
                font-size: 10px;
                line-height: 15px;
                color: rgba(0, 0, 0, 0.4);
                margin: 0;
                white-space: nowrap;
            }
        }

        &__progress {
            background: #dcdde1;
            width: 4px;
            height: 33%;
            margin-left: 8px;
        }
    }

    .checked {
        font-family: 'font_bold', sans-serif;
        font-weight: bold;
        color: #000;
    }

    .blue {
        background: #0068dc;
    }
</style>
