// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bar-container" :style="params.containerStyle">
        <div class="bar-container__fill" :style="params.fillStyle"></div>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';

    @Component
    export default class Bar extends Vue {
        @Prop({default: ''})
        private readonly current: string;
        @Prop({default: ''})
        private readonly max: string;
        @Prop({default: '#224CA5'})
        private readonly color: string;
        @Prop({default: '8px'})
        private readonly height: string;

        public get params(): object {
            return {
                fillStyle: {
                    'background-color': this.color,
                    'width': (parseFloat(this.current) / parseFloat(this.max)) * 100 + '%'
                },

                containerStyle: {
                    height: this.height,
                },
            };
        }
    }
</script>

<style scoped lang="scss">
    .bar-container {
        width: 100%;
        margin-top: 11px;
        border-radius: 4px;
        background-color: #F4F6F9;
        position: relative;

        &__fill {
            height: 100%;
            position: absolute;
            left: 0;
            top: 0;
            border-radius: 20px;
        }
    }
</style>

