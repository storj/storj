// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="progress">
        <p class="progress__title">Quickstart Guide</p>
        <h1 class="progress__subtitle">Start with Uplink CLI</h1>
        <div class="progress__bar">
            <div class="progress__bar__fill" :style="{width: fill}" />
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
     * Returns fill bar width depending on current route name.
     */
    public get fill(): string {
        switch (this.$route.name) {
        case RouteConfig.AGName.name:
            return '9%';
        case RouteConfig.AGPermissions.name:
            return '18%';
        case RouteConfig.APIKey.name:
            return '27%';
        case RouteConfig.CLIInstall.name:
            return '36%';
        case RouteConfig.CLISetup.name:
            return '45%';
        case RouteConfig.CreateBucket.name:
            return '54%';
        case RouteConfig.UploadObject.name:
            return '63%';
        case RouteConfig.ListObject.name:
            return '72%';
        case RouteConfig.DownloadObject.name:
            return '81%';
        case RouteConfig.ShareObject.name:
            return '90%';
        case RouteConfig.SuccessScreen.name:
            return '100%';
        default:
            return '0%';
        }
    }
}
</script>

<style scoped lang="scss">
    .progress {
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;
        margin-bottom: 30px;

        &__title {
            font-size: 16px;
            line-height: 24px;
            color: #000;
        }

        &__subtitle {
            font-size: 28px;
            line-height: 36px;
            letter-spacing: -0.02em;
            color: #000;
            margin-bottom: 16px;
            font-family: 'font_bold', sans-serif;
        }

        &__bar {
            width: 580px;
            height: 16px;
            background: #d8dee3;
            border-radius: 8px;

            &__fill {
                height: 16px;
                background: #00ac26;
                border-radius: 8px;
                transition: width 0.2s;
            }
        }
    }
</style>
