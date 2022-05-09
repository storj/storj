// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pt-bar">
        <p>
            Upload up to 75TB by upgrading to a Storj Pro Account.
        </p>
        <p class="pt-bar__functional" @click.stop="openAddPMModal">
            Upgrade now.
        </p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class PaidTierBar extends Vue {
    @Prop({default: () => () => false})
    public readonly openAddPMModal: () => void;
}
</script>

<style scoped lang="scss">
    .pt-bar {
        font-family: 'font_regular', sans-serif;
        display: flex;
        align-items: center;
        justify-content: space-between;
        background: #0047ff;
        font-size: 14px;
        line-height: 18px;
        color: #eee;
        padding: 5px 30px;

        &__functional {
            font-family: 'font_bold', sans-serif;
            cursor: pointer;
        }
    }
</style>
