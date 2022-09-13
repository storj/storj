// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="loader">
        <LoaderImage :class="{ white: isWhite }" :style="style" />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import LoaderImage from '@/../static/images/common/loader.svg';

// @vue/component
@Component({
    components: {
        LoaderImage,
    },
})
export default class VLoader extends Vue {
    @Prop({ default: 'unset' })
    private readonly width: string;
    @Prop({ default: 'unset' })
    private readonly height: string;
    @Prop({ default: false })
    private readonly isWhite: boolean;

    /**
     * Returns loader's width and height from props.
     */
    public get style(): Record<string, unknown> {
        return { width: this.width, height: this.height };
    }
}
</script>

<style scoped lang="scss">
    .loader {
        width: 100%;
        display: flex;
        justify-content: center;
    }

    .white :deep(path) {
        fill: #fff;
    }
</style>
