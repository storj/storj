// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div class="overlay">
		<div class="overlay__main-container">
			<div class="overlay__main-container__svg" v-if="nodeStatus.isNone">
				<div class="loading-line"></div>
				<svg width="120px" height="120px" viewBox="0 0 120 120" version="1.1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">
					<defs>
						<linearGradient x1="65.4452463%" y1="208.17803%" x2="167.766742%" y2="150.69682%" id="linearGradient-1">
							<stop stop-color="#4381F7" offset="0%"></stop>
							<stop stop-color="#505460" offset="100%"></stop>
						</linearGradient>
					</defs>
					<g id="Icon/node/1" stroke="none" stroke-width="1" fill="none" fill-rule="evenodd">
						<g id="Group" transform="translate(23.000000, 33.000000)" fill-rule="nonzero">
							<g id="Group-2" transform="translate(0.900000, 0.900000)" fill="#505460">
								<path d="M5.24781681,25 L66.7017485,25 L57.2767486,9.5639393 C55.2407064,6.22649392 51.6542832,4.2 47.7985861,4.2 L22.6740614,4.2 C18.517928,4.2 14.702343,6.55052798 12.7764624,10.3064151 L5.24781681,25 Z M68,29.2 L4.2,29.2 L4.2,36.6374474 C4.2,42.9203342 9.20330161,48 15.3601658,48 L56.8398342,48 C62.9966984,48 68,42.9203342 68,36.6374474 L68,29.2 Z M72.2,27.4952314 L72.2,36.6374474 C72.2,45.2258089 65.3306541,52.2 56.8398342,52.2 L15.3601658,52.2 C6.86934588,52.2 -1.70530257e-13,45.2258089 -1.70530257e-13,36.6374474 L-1.70530257e-13,27.3605891 C-1.70530257e-13,26.4842425 0.207498838,25.6228069 0.605692679,24.8495457 L9.03884835,8.39062775 C11.6812091,3.23744801 16.9365211,0 22.6740614,0 L47.7985861,0 C53.1230697,0 58.0658931,2.7929223 60.861791,7.37591038 L71.3962549,24.6290296 C71.9217493,25.4919082 72.2,26.4841461 72.2,27.4952314 Z" id="Combined-Shape"></path>
							</g>
							<path d="M61,36.9 C62.159798,36.9 63.1,37.840202 63.1,39 C63.1,40.159798 62.159798,41.1 61,41.1 L49,41.1 C47.840202,41.1 46.9,40.159798 46.9,39 C46.9,37.840202 47.840202,36.9 49,36.9 L61,36.9 Z" id="Stroke-5" fill="url(#linearGradient-1)"></path>
						</g>
					</g>
				</svg>
			</div>
			<Success v-if="nodeStatus.isActive"/>
			<Failure v-if="nodeStatus.isError"/>
		</div>
	</div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Success from './Success.vue';
import Failure from './Failure.vue';
import { NodeStatus } from '../types/nodeStatus';

@Component({
    mounted() {
        (document as any).querySelector('.overlay').classList.add('active');
    },
    data: function () {
        return {
            isSuccessCheck: false,
            isFailureCheck: false,
        };
    },
    computed: {
        nodeStatus: function () {
            const currentNodeStatus = this.$store.state.nodeStatus;

            const isNone = currentNodeStatus === NodeStatus.None;
            const isActive = currentNodeStatus === NodeStatus.Active;
            const isError = currentNodeStatus === NodeStatus.Error;

            return {
                isNone,
                isActive,
                isError,
            };
        },
    },
    components: {
        Success,
        Failure,
    },
})

export default class Loading extends Vue {}
</script>

<style lang="scss">
	.overlay {
		position: absolute;
		top: 0;
		left: 0;
		height: 100%;
		width: 0px;
		display: flex;
		justify-content: center;
		align-items: center;
		background-color: transparent;
		-webkit-transition: all 0.5s linear;
		-moz-transition: all 0.5s linear;
		-o-transition: all 0.5s linear;
		transition: all 0.5s linear;

		&__main-container {
			width: auto;
			height: auto;
			visibility: hidden;
			display: flex;
			align-items: center;
			justify-content: center;
			flex-direction: column;
			opacity: 0;
			-webkit-transition: all 1s linear;
			-moz-transition: all 1s linear;
			-o-transition: all 1s linear;
			transition: all 1s linear;
			transition-delay: 1s;

			&__svg {
				position: relative;
			}

			.loading-line {
				height: 4px;
				position: absolute;
				top: 59px;
				left: 28px;
				width: 64px;
				-webkit-transition: all 1s linear;
				-moz-transition: all 1s linear;
				-o-transition: all 1s linear;
				transition: all 1s linear;
				animation-delay: 5s;
				background-color: #1494ff;
				border-radius: 12px;
			}

			h1 {
				margin-top: 33px;
			}

			&__button {
				width: 176px;
				height: 52px;
				border-radius: 8px;
				background-color: #1494ff;
				margin-top: 46px;

				&__text {
					font-family: 'font_bold';
					font-size: 16px;
					font-style: normal;
					font-stretch: normal;
					line-height: normal;
					letter-spacing: normal;
					text-align: center;
					color: #f3f4f9;
				}

				&:hover {
					cursor: pointer;
					background-color: #2039df;
				}
			}

			&__support {
				margin-top: 128px;
				font-family: 'font_regular';
				font-size: 12px;
				font-style: normal;
				font-stretch: normal;
				line-height: 23px;
				letter-spacing: normal;
				text-align: center;
				color: #696c77;

				a {
					font-family: 'font_medium';
					font-size: 12px;
					color: #1494ff;
					text-decoration: none;
					cursor: pointer;
				}
			}

			.loading {
				-webkit-transition: all 1s linear;
				-moz-transition: all 1s linear;
				-o-transition: all 1s linear;
				transition: all 5s linear;
				animation-delay: 2s;
				animation-duration: 2s;
			}
		}
	}
	.overlay.active {
		background-color: #191919;
		width: 100%;
		z-index: 9999;

		.overlay__main-container {
			opacity: 1;
			visibility: visible;
		}

		.loading-line {
			animation: pathWidth 3s linear;
		}
	}

	@keyframes pathWidth {
		from {
			width: 0px;
		}
		to {
			width: 64px;
		}
	}
</style>
