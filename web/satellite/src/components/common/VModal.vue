// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="mask">
        <div class="mask__wrapper">
            <div class="mask__wrapper__container">
                <slot name="content" />
                <div class="mask__wrapper__container__close" @click="onClose">
                    <CloseCrossIcon />
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

// @vue/component
@Component({
    components: {
        CloseCrossIcon,
    },
})
export default class VModal extends Vue {
    @Prop({ default: () => () => {} })
    public readonly onClose: () => void;
}
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
            overflow-y: auto;
            text-align: center;
            padding: 20px 0;

            &__container {
                display: inline-block;
                position: relative;
                background: #fff;
                border-radius: 10px;
                box-shadow: 0 0 32px rgb(0 0 0 / 4%);
                margin: 0 24px;

                @media screen and (max-width: 400px) {
                    margin: 0;
                }

                &__close {
                    position: absolute;
                    right: 24px;
                    top: 24px;
                    cursor: pointer;
                    opacity: 0.55;
                }
            }
        }
    }
</style>
