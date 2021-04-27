// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div :style="style" class="loader"/>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

@Component
export default class VLoader extends Vue {
    @Prop({ default: 'inherit' })
    private readonly width: string;
    @Prop({ default: 'inherit' })
    private readonly height: string;

    /**
     * Returns loader's width and height from props.
     */
    public get style(): Object {
        return { width: this.width, height: this.height };
    }
}
</script>

<style scoped lang="scss">
    .loader {
        border: 16px solid #f3f3f3;
        border-top: 16px solid #3498db;
        border-radius: 50%;
        animation: spin 2s linear infinite;
    }

    @keyframes spin {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
    }
</style>
