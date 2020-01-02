// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="limits-container">
        <div class="limits-container__item">
            <p class="limits-container__item__title">Egress limits used</p>
            <div class="limits-container__item__values-container">
                <p class="limits-container__item__values-container__remaining">{{ bandwidthUsed }}</p>
                <p class="limits-container__item__values-container__total">/  {{ bandwidthLimit }}</p>
            </div>
        </div>
        <div class="limits-container__item">
            <p class="limits-container__item__title">Storage limits used</p>
            <div class="limits-container__item__values-container">
                <p class="limits-container__item__values-container__remaining">{{ storageUsed }}</p>
                <p class="limits-container__item__values-container__total">/  {{ storageLimit }}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { Dimensions, Size } from '@/utils/bytesSize';

@Component
export default class ProjectLimitsArea extends Vue {
    public get bandwidthUsed(): string {
        const bandwidthUsed = new Size(this.$store.getters.selectedProject.limits.bandwidthUsed);

        return this.getFormattedLimit(bandwidthUsed);
    }

    public get bandwidthLimit(): string {
        const bandwidthLimit = new Size(this.$store.getters.selectedProject.limits.bandwidthLimit);

        return this.getFormattedLimit(bandwidthLimit);
    }

    public get storageUsed(): string {
        const storageUsed = new Size(this.$store.getters.selectedProject.limits.storageUsed);

        return this.getFormattedLimit(storageUsed);
    }

    public get storageLimit(): string {
        const storageLimit = new Size(this.$store.getters.selectedProject.limits.storageLimit);

        return this.getFormattedLimit(storageLimit);
    }

    private getFormattedLimit(limit: Size): string {
        switch (limit.label) {
            case Dimensions.Bytes:
            case Dimensions.KB:
                return '0';
            default:
                return `${limit.formattedBytes.replace(/\\.0+$/, '')} ${limit.label}`;
        }
    }
}
</script>

<style scoped lang="scss">
    .limits-container {
        width: 100%;
        display: flex;
        justify-content: space-between;
        align-items: center;

        &__item {
            padding: 37px 28px;
            width: calc(49% - 56px);
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: flex-start;
            background-color: #fff;
            border-radius: 6px;

            &__title {
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                text-align: left;
                color: #afb7c1;
                margin: 0;
            }

            &__values-container {
                display: flex;
                align-items: flex-start;
                justify-content: center;
                margin-top: 10px;

                &__remaining {
                    font-family: 'font_bold', sans-serif;
                    font-size: 36px;
                    color: #39464f;
                    margin: 0;
                }

                &__total {
                    font-family: 'font_medium', sans-serif;
                    font-size: 36px;
                    color: #afb7c1;
                    margin: 0 0 0 15px;
                }
            }
        }
    }
</style>