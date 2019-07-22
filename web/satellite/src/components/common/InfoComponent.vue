// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info" @mouseenter="toggleVisibility" @mouseleave="toggleVisibility">
        <slot></slot>
        <div class="info__message-box" v-if="isVisible">
            <div class="info__message-box__text">
                <p class="info__message-box__text__regular-text">{{text}}</p>
                <p class="info__message-box__text__bold-text">{{boldText}}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';

    @Component
    export default class InfoComponent extends Vue {
        private isVisible: boolean = false;

        @Prop({default: ''})
        private readonly text: String;
        @Prop({default: ''})
        private readonly boldText: String;

        public toggleVisibility(): void {
            this.isVisible = !this.isVisible;
        }
    }
</script>

<style scoped lang="scss">
    .info {
        position: relative;

        &__message-box {
            position: absolute;
            left: 40%;
            bottom: 110%;
            transform: translate(-50%);
            height: auto;
            width: auto;
            min-width: 180px;
            display: flex;
            justify-content: center;
            align-items: center;
            text-align: center;
            background-image: url('../../../static/images/Message.png');
            background-size:100% 100%;
            z-index: 101;
            padding: 11px 18px 20px 18px;

            &__text {
                align-items: center;
                justify-content: center;
                font-size: 12px;
                line-height: 16px;

                &__bold-text {
                    color: #586C86;
                    font-family: 'font_bold';
                    margin-block-start: 0;
                    margin-block-end: 0;
                }

                &__regular-text {
                    color: #5A6E87;
                    font-family: 'font_regular';
                    margin-block-start: 0;
                    margin-block-end: 0;
                }
            }
        }
    }
</style>
