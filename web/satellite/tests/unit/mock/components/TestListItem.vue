// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <h1 class="title">{{ itemData }}</h1>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class TestListItem extends Vue {
    @Prop({ default: 'a' })
    public itemData: string;
}
</script>

<style scoped lang="scss">
    .title {
        font-size: 15px;
    }
</style>
