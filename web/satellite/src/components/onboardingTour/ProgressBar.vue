// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="progress-bar-container">
        <div class="progress-bar-container__progress-area">
            <div
                class="progress-bar-container__progress-area__circle"
                :class="{ 'completed-step': isCreateProjectStep || isCreateApiKeyStep || isUploadDataStep }"
            >
                <CheckedImage/>
            </div>
            <div
                class="progress-bar-container__progress-area__bar"
                :class="{ 'completed-step': isCreateApiKeyStep || isUploadDataStep }"
            />
            <div
                class="progress-bar-container__progress-area__circle"
                :class="{ 'completed-step': isCreateApiKeyStep || isUploadDataStep }"
            >
                <CheckedImage/>
            </div>
            <div
                class="progress-bar-container__progress-area__bar"
                :class="{ 'completed-step': isUploadDataStep }"
            />
            <div
                class="progress-bar-container__progress-area__circle"
                :class="{ 'completed-step': isUploadDataStep }"
            >
                <CheckedImage/>
            </div>
        </div>
        <div class="progress-bar-container__titles-area">
            <span
                class="progress-bar-container__titles-area__title"
                :class="{ 'completed-font-color': isCreateProjectStep || isCreateApiKeyStep || isUploadDataStep }"
            >
                Name Your Project
            </span>
            <span
                class="progress-bar-container__titles-area__title api-key-title"
                :class="{ 'completed-font-color': isCreateApiKeyStep || isUploadDataStep }"
            >
                Create an API Key
            </span>
            <span
                class="progress-bar-container__titles-area__title"
                :class="{ 'completed-font-color': isUploadDataStep }"
            >
                Upload Data
            </span>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import CheckedImage from '@/../static/images/common/checked.svg';

@Component({
    components: {
        CheckedImage,
    },
})

export default class ProgressBar extends Vue {
    @Prop({ default: false })
    public readonly isCreateProjectStep: boolean;
    @Prop({ default: false })
    public readonly isCreateApiKeyStep: boolean;
    @Prop({ default: false })
    public readonly isUploadDataStep: boolean;
}
</script>

<style scoped lang="scss">
    .progress-bar-container {
        width: 100%;

        &__progress-area {
            width: calc(100% - 420px);
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 25px 210px 6px 210px;

            &__circle {
                display: flex;
                justify-content: center;
                align-items: center;
                min-width: 20px;
                height: 20px;
                background-color: #c5cbdb;
                border-radius: 10px;
            }

            &__bar {
                width: 100%;
                height: 4px;
                background-color: #c5cbdb;
            }
        }

        &__titles-area {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 0 188px 0 178px;

            &__title {
                font-family: 'font_regular', sans-serif;
                font-size: 10px;
                line-height: 15px;
                color: rgba(0, 0, 0, 0.4);
                text-align: center;
            }
        }
    }

    .api-key-title {
        padding: 0 15px 0 0;
    }

    .completed-step {
        background-color: #2683ff;
    }

    .completed-font-color {
        color: #2683ff;
    }

    @media screen and (max-width: 800px) {

        .progress-bar-container {

            &__progress-area {
                width: calc(100% - 300px);
                padding: 25px 150px 6px 150px;
            }

            &__titles-area {
                padding: 0 128px 0 118px;
            }
        }
    }
</style>