// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dropzone" @mouseout="close" @mouseleave="close" @dragleave.self="close">
        <div class="dropzone__message">
            <p class="dropzone__message__text">
                Drop your files to put it into the “{{ bucket }}” bucket.
            </p>
        </div>
        <p class="dropzone__info">Drag and drop files here to upload</p>
    </div>
</template>

<script setup lang="ts">
const props = defineProps<{
    bucket: string
    close: () => void
}>();
</script>

<style scoped lang="scss">
.dropzone {
    z-index: 1;
    position: fixed;
    inset: 0;
    background: rgb(0 0 0 / 35%);
    border: 1px dashed var(--c-white);
    display: flex;
    align-items: center;
    justify-content: center;

    &__message {
        padding: 10px 24px;
        background: var(--c-green-1);
        border: 1px solid var(--c-green-5);
        border-radius: 8px;
        position: absolute;
        top: 24px;
        pointer-events: none;

        &__text {
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            line-height: 20px;
            color: var(--c-green-5);
            text-align: center;
        }
    }

    &__info {
        font-family: 'font_bold', sans-serif;
        font-size: 40px;
        line-height: 50px;
        text-align: center;
        max-width: 380px;
        color: var(--c-white);
        text-shadow: 0 7px 20px 0 rgb(0 0 0 / 15%);
        pointer-events: none;
    }
}
</style>
