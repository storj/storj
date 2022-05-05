// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="flow-container">
        <slot name="icon" />
        <h1 class="flow-container__title" aria-roledescription="title">{{ title }}</h1>
        <slot name="content" />
        <div class="flow-container__buttons">
            <VButton
                class="flow-container__buttons__back"
                label="< Back"
                height="64px"
                border-radius="52px"
                is-grey-blue="true"
                :on-press="onBackClick"
                :is-disabled="isLoading"
            />
            <VButton
                label="Next >"
                height="64px"
                border-radius="52px"
                :on-press="onNextClick"
                :is-disabled="isLoading"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VButton from "@/components/common/VButton.vue";

// @vue/component
@Component({
    components: {
        VButton,
    },
})
export default class CLIFlowContainer extends Vue {
    @Prop({ default: () => () => {}})
    public readonly onNextClick: () => unknown;
    @Prop({ default: () => () => {}})
    public readonly onBackClick: () => unknown;
    @Prop({ default: ''})
    public readonly title: string;
    @Prop({ default: false})
    public readonly isLoading: boolean;
}
</script>

<style scoped lang="scss">
    .flow-container {
        font-family: 'font_regular', sans-serif;
        background: #fff;
        box-shadow: 0 0 32px rgb(0 0 0 / 4%);
        border-radius: 20px;
        padding: 48px;
        max-width: 500px;

        &__title {
            margin: 20px 0;
            font-family: 'font_Bold', sans-serif;
            font-size: 48px;
            line-height: 56px;
            letter-spacing: 1px;
            color: #14142b;
        }

        &__buttons {
            display: flex;
            align-items: center;
            width: 100%;
            margin-top: 48px;

            &__back {
                margin-right: 24px;
            }
        }
    }
</style>
