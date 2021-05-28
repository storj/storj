// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="warning-view">
        <div class="warning-view__container">
            <h1 class="warning-view__container__title">Object Browser</h1>
            <div class="warning-view__container__message-container">
                <h2 class="warning-view__container__message-container__sub-title">The object browser uses server side encryption.</h2>
                <p class="warning-view__container__message-container__message">
                    If you want to use our product with only end-to-end encryption, you may want to skip this feature.
                </p>
            </div>
            <div class="warning-view__container__buttons-area">
                <VButton
                    class="warning-view__container__buttons-area__left-button"
                    label="Return to dashboard"
                    width="50%"
                    height="48px"
                    :on-press="goToDashboard"
                    :is-blue-white="true"
                />
                <VButton
                    label="Continue"
                    width="50%"
                    height="48px"
                    :on-press="proceed"
                />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import { RouteConfig } from '@/router';

@Component({
    components: {
        VButton,
    },
})
export default class WarningView extends Vue {
    /**
     * Redirects to project dashboard page.
     */
    public goToDashboard(): void {
        this.$router.push(RouteConfig.ProjectDashboard.path);
    }

    /**
     * Proceeds further into file browser flow.
     */
    public proceed(): void {
        this.$router.push(RouteConfig.CreatePassphrase.path);
    }
}
</script>

<style scoped lang="scss">
    .warning-view {
        font-family: 'font_regular', sans-serif;
        font-style: normal;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 200px 0 20px 0;

        &__container {
            background: #fff;
            border-radius: 6px;
            padding: 35px 50px;
            max-width: 515px;

            &__title {
                width: 100%;
                text-align: center;
                font-family: 'font_Bold', sans-serif;
                font-size: 22px;
                line-height: 27px;
                color: #000;
                margin: 0 0 35px 0;
            }

            &__message-container {
                padding: 40px 30px;
                width: calc(100% - 60px);
                background: #fff;
                border: 1px solid #e6e9ef;
                border-radius: 9px;
                display: flex;
                flex-direction: column;
                align-items: center;

                &__sub-title {
                    font-family: 'font_bold', sans-serif;
                    font-size: 16px;
                    line-height: 19px;
                    text-align: center;
                    color: #ce3030;
                    margin: 0 0 10px 0;
                }

                &__message {
                    font-weight: normal;
                    font-size: 16px;
                    line-height: 24px;
                    text-align: center;
                    color: #1b2533;
                    word-break: break-word;
                    margin: 0;
                }
            }

            &__buttons-area {
                margin-top: 35px;
                display: flex;
                align-items: center;
                justify-content: space-between;

                &__left-button {
                    margin-right: 40px;
                }
            }
        }
    }
</style>
