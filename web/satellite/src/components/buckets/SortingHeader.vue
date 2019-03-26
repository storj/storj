// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<tr class="sort-header-container">
		<th class="sort-header-container__item">
			<div class="row">
				<p>Bucket Name</p>
				<div class="sort-header-container__item__arrows">
					<span v-html="arrowUp"></span>
					<span class="selected" v-html="arrowDown"></span>
				</div>
			</div>
		</th>
		<th class="sort-header-container__item">
			<div class="row">
				<p>Creation Date</p>
				<div class="sort-header-container__item__arrows">
					<span v-html="arrowUp"></span>
					<span v-html="arrowDown"></span>
				</div>
			</div>
		</th>
		<th class="sort-header-container__item">
			<div class="row">
				<p>Storage Used</p>
				<div class="sort-header-container__item__arrows">
					<span v-html="arrowUp"></span>
					<span v-html="arrowDown"></span>
				</div>
			</div>
		</th>
		<th class="sort-header-container__item">
			<div class="row">
				<p>Egress Used</p>
				<div class="sort-header-container__item__arrows">
					<span v-html="arrowUp"></span>
					<span v-html="arrowDown"></span>
				</div>
			</div>
		</th>
		<th class="sort-header-container__item">
			<div class="row">
				<p>Objects Stored</p>
				<div class="sort-header-container__item__arrows">
					<span v-html="arrowUp"></span>
					<span v-html="arrowDown"></span>
				</div>
			</div>
		</th>
	</tr>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
    @Component({
        data: function() {
            return {
                arrowUp: EMPTY_STATE_IMAGES.ARROW_UP,
                arrowDown: EMPTY_STATE_IMAGES.ARROW_DOWN,
            };
        }
    })
    export default class SortBucketsHeader extends Vue {
    }
</script>

<style scoped lang="scss">
	.sort-header-container {
		height: 80px;

		&__item {
			margin: 0;
			cursor: pointer;
			text-align: left;
			padding-left: 30px;
			&__arrows {
				display: flex;
				flex-direction: column;
				justify-content: flex-start;
				padding-bottom: 12px;
				margin-left: 10px;
				span.selected {

					svg {
						path {
							fill: #2683FF !important;
						}
					}
				}
				span {
					height: 10px;
				}
			}
			p {
				font-family: 'font_medium';
				font-size: 16px;
				line-height: 23px;
				color: #AFB7C1;
			}

			&:nth-child(1) {
				margin-left: 0px;
			}
		}
	}
	.row {
		display: flex;
		flex-direction: row;
		align-items: center;
	}
	@media screen and (max-width: 1600px) {
		.sort-header-container {
			grid-template-columns: 25% 28% 19% 15% 14%;
			height: 65px;
		}
	}
</style>
