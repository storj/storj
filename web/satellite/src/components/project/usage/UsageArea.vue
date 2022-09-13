// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="usage-area">
        <p class="usage-area__title">{{ title }}</p>
        <VLoader v-if="isDataFetching" />
        <template v-else>
            <pre class="usage-area__remaining">{{ remainingFormatted }} Remaining</pre>
            <VBar
                :current="used"
                :max="limit"
                color="#0068DC"
            />
            <div class="usage-area__limits-area">
                <pre class="usage-area__limits-area__title">{{ title }} Used</pre>
                <pre class="usage-area__limits-area__limits">{{ usedFormatted }} / {{ limitFormatted }}</pre>
            </div>
        </template>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { Dimensions, Size } from '@/utils/bytesSize';

import VBar from '@/components/common/VBar.vue';
import VLoader from '@/components/common/VLoader.vue';

// @vue/component
@Component({
    components: {
        VBar,
        VLoader,
    },
})
export default class UsageArea extends Vue {
    @Prop({ default: '' })
    public readonly title: string;
    @Prop({ default: 0 })
    public readonly used: number;
    @Prop({ default: 0 })
    public readonly limit: number;
    @Prop({ default: true })
    public readonly isDataFetching: boolean;

    /**
     * Returns formatted remaining amount.
     */
    public get remainingFormatted(): string {
        const remaining = this.limit - this.used;
        if (remaining < 0) {
            return '0 Bytes';
        }

        return this.formattedValue(new Size(remaining, 2));
    }

    /**
     * Returns formatted used amount.
     */
    public get usedFormatted(): string {
        return this.formattedValue(new Size(this.used, 2));
    }

    /**
     * Returns formatted limit amount.
     */
    public get limitFormatted(): string {
        return this.formattedValue(new Size(this.limit, 2));
    }

    /**
     * Formats value to needed form and returns it.
     */
    private formattedValue(value: Size): string {
        switch (value.label) {
        case Dimensions.Bytes:
        case Dimensions.KB:
            return '0';
        default:
            return `${value.formattedBytes.replace(/\\.0+$/, '')}${value.label}`;
        }
    }
}
</script>

<style scoped lang="scss">
    h1,
    p,
    pre {
        margin: 0;
    }

    .usage-area {
        padding: 20px;
        border-radius: 6px;
        background-color: #fff;

        &__title {
            font-size: 16px;
            line-height: 16px;
            color: #1b2533;
            margin-bottom: 25px;
            font-family: 'font_bold', sans-serif;
        }

        &__remaining {
            font-size: 16px;
            line-height: 16px;
            color: #1b2533;
            margin-bottom: 15px;
            font-family: 'font_regular', sans-serif;
        }

        &__limits-area {
            width: 100%;
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-top: 15px;

            &__title {
                font-size: 14px;
                line-height: 14px;
                color: #384761;
                font-family: 'font_regular', sans-serif;
            }

            &__limits {
                font-size: 14px;
                line-height: 14px;
                color: #1b2533;
                font-family: 'font_regular', sans-serif;
            }
        }
    }
</style>
