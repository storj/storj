// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div ref="modal" class="mask" tabindex="0" @keydown.esc="closeHandler">
        <div class="mask__wrapper" @click.self="closeHandler">
            <div class="mask__wrapper__container">
                <slot name="content" />
                <div v-if="isClosable" class="mask__wrapper__container__close" @click="closeHandler">
                    <CloseCrossIcon />
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

const props = withDefaults(defineProps<{
    onClose?: () => void;
    isClosable?: boolean;
}>(), {
    onClose: () => () => {},
    isClosable: true,
});

const modal = ref<HTMLElement>();

/**
 * Holds on close modal logic.
 */
function closeHandler(): void {
    if (props.isClosable) {
        props.onClose();
    }
}

onMounted((): void => {
    modal.value?.focus();
});
</script>

<style scoped lang="scss">
    .mask {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        z-index: 9999;
        background: rgb(27 37 51 / 75%);

        &__wrapper {
            height: calc(100% - 40px);
            max-height: calc(100vh - 40px);
            overflow: hidden auto;
            text-align: center;
            padding: 20px 0;

            &__container {
                display: inline-block;
                position: relative;
                background: #fff;
                border-radius: 10px;
                box-shadow: 0 0 32px rgb(0 0 0 / 4%);
                margin: 0 24px;

                @media screen and (width <= 400px) {
                    margin: 0;
                }

                &__close {
                    position: absolute;
                    right: 3px;
                    top: 3px;
                    padding: 10px;
                    border-radius: 16px;
                    cursor: pointer;

                    &:hover {
                        background-color: var(--c-grey-2);
                    }

                    &:active {
                        background-color: var(--c-grey-4);
                    }

                    svg {
                        display: block;
                        width: 12px;
                        height: 12px;

                        :deep(path) {
                            fill: var(--c-black);
                        }
                    }
                }
            }
        }
    }
</style>
