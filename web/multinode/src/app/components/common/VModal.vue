// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="modal-wrap" @click.self.stop="close">
        <div class="modal">
            <div class="modal__header">
                <slot name="header" />
            </div>
            <div class="modal__body">
                <slot name="body" />
            </div>
            <div class="modal__footer">
                <slot name="footer" />
            </div>
            <div class="modal__cross" @click.stop="close">
                <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32" fill="none">
                    <path d="M24 8L8 24" stroke="#676F84" stroke-width="2.66667" stroke-linecap="round" stroke-linejoin="round" />
                    <path d="M8 8L24 24" stroke="#676F84" stroke-width="2.66667" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
const emit = defineEmits<{
    (e: 'onClose'): void;
}>();

function close(): void {
    emit('onClose');
}
</script>

<style lang="scss">
    .modal-wrap {
        position: fixed;
        inset: 0;
        background-color: rgb(37 42 50 / 70%);
        z-index: 1000;
        display: flex;
        justify-content: center;
        align-items: center;
        flex-direction: column;
        font-family: 'font_regular', sans-serif;

        .modal {
            position: relative;
            background: var(--v-background-base);
            padding: 80px 97px;
            height: auto;
            z-index: 1001;
            border-radius: 16px;

            &__header {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                color: var(--v-header-base);
                display: flex;
                justify-content: center;
            }

            &__body {
                margin-top: 46px;
            }

            &__footer {
                margin-top: 32px;
            }

            &__cross {
                position: absolute;
                top: 32px;
                right: 32px;
                display: flex;
                align-items: center;
                justify-content: center;
                width: 32px;
                max-width: 32px;
                height: 32px;
                max-height: 32px;
                cursor: pointer;
            }
        }
    }
</style>
