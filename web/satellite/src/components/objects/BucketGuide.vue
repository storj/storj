// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bucket-guide" @click.stop>
        <p class="bucket-guide__title">Upload</p>
        <p class="bucket-guide__content">To upload files, open an existing bucket or create a new one.</p>
        <div class="bucket-guide__actions">
            <div class="bucket-guide__actions__primary" @click.stop="hideGuide">
                <p class="bucket-guide__actions__primary__label">OK</p>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
const props = withDefaults(defineProps<{
    hideGuide: () => void;
}>(), {
    hideGuide: () => {},
});
</script>

<style scoped lang="scss">
    .bucket-guide {
        position: absolute;
        top: 25px;
        left: -26px;
        width: 25rem;
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        background-color: #0149ff;
        text-align: center;
        border-radius: 8px;
        box-sizing: border-box;
        padding: 20px;
        z-index: 1001;
        cursor: default;
        font-family: 'font_regular', sans-serif;

        &:after {
            content: '';
            position: absolute;
            bottom: 100%;
            left: 20px;
            border-width: 10px;
            border-style: solid;
            border-color: #0149ff transparent transparent;
            transform: rotate(180deg);
        }

        &__title {
            font-weight: 400;
            font-size: 12px;
            line-height: 18px;
            color: white;
            opacity: 0.5;
            margin-bottom: 4px;
        }

        &__content {
            color: white;
            font-weight: 500;
            font-size: 15px;
            line-height: 24px;
            margin-bottom: 8px;
            white-space: initial;
            text-align: start;
        }

        &__actions {
            display: flex;
            justify-content: flex-end;
            align-items: center;
            width: 100%;

            &__primary {
                padding: 6px 12px;
                border-radius: 8px;
                background-color: white;
                margin-left: 10px;
                cursor: pointer;

                &__label {
                    font-weight: 700;
                    font-size: 13px;
                    line-height: 20px;
                    color: #0149ff;
                }
            }
        }
    }
</style>
