// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="truncation-container">
        <div class="first-part-text">{{ textFirstPart }}</div>
        <div class="last-part-text">{{ textLastPart }}</div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class MiddleTruncate extends Vue {
    @Prop()
    private readonly text: string;

    public get textFirstPart(): string {
        return this.text.substring(0, this.text.length - 5);
    }

    public get textLastPart(): string {
        return this.text.substring(this.text.length - 5);
    }
}
</script>

<style scoped lang="scss">
.truncation-container {
    display: inline-flex;
    max-width: 30ch;
    min-width: 0;

    &:hover {
        max-width: unset;
        min-width: auto;
    }
}

.first-part-text {
    flex-grow: 1;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
}

.last-part-text {
    flex-shrink: 0;
}
</style>