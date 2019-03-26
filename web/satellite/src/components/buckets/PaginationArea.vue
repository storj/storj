// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div>
		<div class="pagination-container">
			<div class="pagination-container__pages">
				<div v-html="arrowLeft" class="pagination-container__button"></div>
				<div class="pagination-container__items">
					<span class="selected">1</span>
					<span>2</span>
					<span>3</span>
					<span>4</span>
					<span>5</span>
				</div>
				<div v-html="arrowRight" class="pagination-container__button"></div>
			</div>
			<div class="pagination-container__counter">
				<p>Showing <span>1</span> to <span>6</span> of <span>90</span> entries.</p>
			</div>
		</div>
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

    @Component({
        data: function() {
            return {
                arrowLeft: EMPTY_STATE_IMAGES.ARROW_LEFT,
                arrowRight: EMPTY_STATE_IMAGES.ARROW_RIGHT,
            };
        },
    })

    export default class PaginationArea extends Vue {}
</script>

<style scoped lang="scss">
	.pagination-container {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding-left: 25px;
		margin-top: 25px;
		&__pages {
			display: flex;
			align-items: center;
		}
		&__counter {

			p {
				font-family: 'font_medium';
				font-size: 16px;
				color: #AFB7C1;
			}
		}
		&__button {
			display: flex;
			align-items: center;
			justify-content: center;
			cursor: pointer;
			border: 1px solid #AFB7C1;
			border-radius: 6px;
			width: 30px;
			height: 30px;
			&:hover {
				svg {
					path {
						fill: #fff !important;
					}
				}
			}
		}
		&__items {
			margin: 0 20px;
			display: flex;
			.selected {
				color: #2379EC;
				font-family: 'font_bold';
				&:after {
					content: '';
					display: block;
					position: absolute;
					bottom: -4px;
					left: 0;
					width: 10px;
					height: 2px;
					background-color: #2379EC;
				}
			}
			span {
				font-family: 'font_medium';
				font-size: 16px;
				margin-right: 15px;
				cursor: pointer;
				display: block;
				position: relative;
				transition: all .2s ease;
				&:hover {
					color: #2379EC;
					&:after {
						content: '';
						display: block;
						position: absolute;
						bottom: -4px;
						left: 0;
						width: 100%;
						height: 2px;
						background-color: #2379EC;
					}
				}
				&:last-child {
					margin-right: 0;
				}
			}
		}
	}
</style>
