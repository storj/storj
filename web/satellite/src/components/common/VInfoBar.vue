// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info-bar" :class="{ blue: isBlue }">
        <div class="info-bar__info-area">
            <b class="info-bar__info-area__first-value">{{ firstValue }}</b>
            <span class="info-bar__info-area__first-description">{{ firstDescription }}</span>
            <b class="info-bar__info-area__second-value">{{ secondValue }}</b>
            <span class="info-bar__info-area__second-description">{{ secondDescription }}</span>
            <router-link v-if="path" class="info-bar__info-area__button" :to="path">Details</router-link>
        </div>
        <a
            class="info-bar__link"
            :class="{ blue: isBlue }"
            :href="link"
            target="_blank"
            rel="noopener noreferrer"
        >
            {{ linkLabel }}
        </a>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

/**
 * VBanner is custom banner on top of all pages in Dashboard
 */
@Component
export default class VInfoBar extends Vue {
    @Prop({default: ''})
    private readonly firstValue: string;
    @Prop({default: ''})
    private readonly secondValue: string;
    @Prop({default: ''})
    private readonly firstDescription: string;
    @Prop({default: ''})
    private readonly secondDescription: string;
    @Prop({default: ''})
    private readonly link: string;
    @Prop({default: ''})
    private readonly linkLabel: string;
    @Prop({default: ''})
    private readonly path: string;
    @Prop({default: false})
    private readonly isBlue: boolean;
}
</script>

<style scoped lang="scss">
    .info-bar {
        position: absolute;
        top: 0;
        left: 0;
        display: flex;
        justify-content: space-between;
        align-items: center;
        background-color: #dce5f2;
        width: calc(100% - 60px);
        padding: 10px 30px;
        font-family: 'font_regular', sans-serif;
        color: #2b3543;

        &__info-area {
            display: flex;
            align-items: center;

            &__first-value,
            &__second-value,
            &__first-description,
            &__second-description {
                margin-right: 5px;
                font-size: 14px;
                line-height: 17px;
            }

            &__button {
                padding: 5px 10px;
                margin-left: 15px;
                background-color: #fff;
                opacity: 0.8;
                border-radius: 6px;
                font-size: 12px;
                line-height: 15px;
                letter-spacing: 0.02em;
                color: rgba(43, 53, 67, 0.5);
            }
        }

        &__link {
            font-size: 14px;
            line-height: 17px;
            font-family: 'font_medium', sans-serif;
            color: #2683ff;
        }
    }

    .blue {
        background-color: #2582ff;
        color: #fff;
    }
</style>
