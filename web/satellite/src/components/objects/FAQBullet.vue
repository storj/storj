// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bullet">
        <div class="bullet__header" @click="toggle">
            <h2 class="bullet__header__title">{{ title }}</h2>
            <MinusIcon v-if="isExpanded" />
            <PlusIcon v-else />
        </div>
        <p v-if="isExpanded" class="bullet__text">{{ text }}</p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import PlusIcon from '@/../static/images/common/plus.svg';
import MinusIcon from '@/../static/images/common/minus.svg';

// @vue/component
@Component({
    components: {
        PlusIcon,
        MinusIcon,
    },
})
export default class FAQBullet extends Vue {
    @Prop({ default: '' })
    public readonly title: string;
    @Prop({ default: '' })
    public readonly text: string;

    public isExpanded = false;

    /**
     * Toggles bullet text visibility.
     */
    public toggle(): void {
        this.isExpanded = !this.isExpanded;
    }
}
</script>

<style scoped lang="scss">
    .bullet {
        background: #fff;
        border: 2px solid #d8dee3;
        border-radius: 16px;
        padding: 28px 32px;
        width: calc(100% - 64px);
        margin-bottom: 30px;

        &__header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            cursor: pointer;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 32px;
                letter-spacing: 0.75px;
                color: #091c45;
            }
        }

        &__text {
            margin-top: 12px;
            font-size: 16px;
            line-height: 32px;
            color: #56606d;
        }
    }
</style>
