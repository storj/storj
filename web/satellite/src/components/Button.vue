<template>
    <div v-bind:class="[isWhite ? 'container white' : 'container']" :style="style" v-on:click="onPress">
        <h1 v-bind:class="[isWhite ? 'label white' : 'label']">{{label}}</h1>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component(
    { 
		props: {
            label: {
                type: String,
                default: 'Default'
            },
            width: {
                type: String,
                default: 'inherit'
            },
            height: {
                type: String,
                default: 'inherit'
            },
            isWhite: {
                type: Boolean,
                default: false
            },
            onPress: {
                type: Function,
                default: () => {}
            }
        },
        computed: {
            style: function () {
                return { width: this.$props.width, height: this.$props.height }
            }
        }
    }
)

export default class Button extends Vue {}
</script>

<style scoped lang="scss">
    .container {
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: #2683FF;
        border-radius: 6px;
        cursor: pointer;
        .label {
            font-family: 'montserrat_medium';
			font-size: 16px;
			line-height: 23px;
            color: #fff;
        }
        .label.white{
            color: #354049;
        }
    }
    .container.white {
        background-color: #fff;
        border: 1px solid #AFB7C1;
    }
</style>
