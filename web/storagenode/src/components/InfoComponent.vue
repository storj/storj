// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div class="info" @mouseenter="toggleVisibility" @mouseleave="toggleVisibility">
		<slot class="slot"></slot>
		<div class="info__message-box" v-if="isVisible" :style="messageBoxStyle">
			<div class="info__message-box__text">
				<p class="info__message-box__text__regular-text">{{text}}</p>
				<p class="info__message-box__text__bold-text">{{boldText}}</p>
			</div>
		</div>
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';

    @Component(
        {
	        data: function() {
	            return {
                    isVisible: false,
		            height: '5px'
	            }
	        },
            props: {
                text: String,
                boldText: String,
            },
	        methods: {
                toggleVisibility: function (): void {
	                this.$data.isVisible = !this.$data.isVisible;
                }
	        },
	        computed: {
				messageBoxStyle: function (): object {
					return {
					    bottom: this.$data.height
					}
                }
	        },
	        mounted() {
                let infoComponent = document.querySelector('.info');
                if(infoComponent) {
                    const slots = this.$slots.default;
                    if(slots) {
						const slot = slots[0];
						if(slot && slot.elm) {
							this.$data.height = (slot.elm as HTMLElement).offsetHeight + 'px';
						}
					}
                }
	        }
        }
    )
    export default class InfoComponent extends Vue {
    }
</script>

<style scoped lang="scss">
	.info {
		position: relative;

		&__message-box {
			position: absolute;
			left: 50%;
			transform: translate(-50%);
			height: auto;
			width: auto;
			display: flex;
			white-space: nowrap;
			justify-content: center;
			align-items: center;
			background-image: url('../../static/images/Message.png');
			background-size:100% 100%;
			z-index: 101;

			&__text {
				margin: 11px 18px 20px 18px;
				display: flex;
				flex-direction: column;
				align-items: center;
				justify-content: center;

				&__bold-text {
					color: #586C86;
					font-size: 12px;
					line-height: 16px;
					font-family: 'font_bold';
					margin-block-start: 0;
					margin-block-end: 0;
				}

				&__regular-text {
					color: #5A6E87;
					font-size: 12px;
					line-height: 16px;
					font-family: 'font_regular';
					margin-block-start: 0;
					margin-block-end: 0;
				}
			}
		}
	}
</style>
