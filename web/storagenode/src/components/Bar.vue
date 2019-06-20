// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div class="bar-container" :style="params.containerStyle">
		<div class="bar-container__fill" :style="params.fillStyle"></div>
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';

    @Component(
        {
            props: {
                current: String,
                max: String,
				color: {
                    type: String,
					default: '#224CA5',
				},
	            height: {
                    type: String,
		            default: '8px',
                },
            },
	        computed: {
                params: function (): any {
	                return {
	                    fillStyle: {
	                        'background-color': this.$props.color,
		                    'width': (parseFloat(this.$props.current) / parseFloat(this.$props.max)) * 100 + '%'
	                    },
		                containerStyle: {
	                        height: this.$props.height
                        }
	                }
                }
	        }
        }
    )
    export default class Bar extends Vue {

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

