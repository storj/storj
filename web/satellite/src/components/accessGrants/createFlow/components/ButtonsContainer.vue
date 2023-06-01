// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="buttons">
        <p v-if="label" class="buttons__label">{{ label }}</p>
        <div class="buttons__row">
            <slot name="leftButton" />
            <slot name="rightButton" />
        </div>
    </div>
</template>

<script setup lang="ts">
const props = withDefaults(defineProps<{
    label?: string;
}>(), {
    label: '',
});
</script>

<style scoped lang="scss">
.buttons {
    margin-top: 16px;

    &__label {
        font-family: 'font_bold', sans-serif;
        font-size: 14px;
        line-height: 20px;
        color: var(--c-grey-6);
        text-align: left;
        margin-bottom: 16px;
    }

    &__row {
        align-items: center;
        column-gap: 16px;
        display: flex;

        @media screen and (width <= 460px) {
            flex-direction: column-reverse;
            column-gap: unset;
            row-gap: 10px;
        }
    }
}
</style>
