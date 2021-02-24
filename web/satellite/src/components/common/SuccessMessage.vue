// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="success-message__wrapper">
        <p class="success-message__text">{{ message }}<p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

@Component
export default class SuccessMessage extends Vue {

    @Prop({default: 'Success!'})
    protected readonly message: string;
}
</script>

<style scoped lang="scss">
    .success-message {

        &__wrapper {
            background: #eafff7;
            border: 1px solid #1a9666;
            box-sizing: border-box;
            border-radius: 6px;
            height: 52px;
            width: 100%;
        }

        &__text {
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 133.12%;
            color: #1a9666;
            position: relative;
            left: 15px;
        }
    }
</style>
