// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info-block">
        <slot name="body" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class InfoBlock extends Vue {}
</script>

<style scoped lang="scss">
    .info-block {
        box-sizing: border-box;
        padding: 30px;
        background: transparent;
        border: 1px solid var(--c-gray--light);
        border-radius: var(--br-block);
        width: 100%;
    }
</style>
