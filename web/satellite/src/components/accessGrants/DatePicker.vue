// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div class="date-picker">
		<div class="date-picker__list">
			<ul class="date-picker__list__column">
				<li class="date-picker__list__column-item">Forever</li>
				<li class="date-picker__list__column-item">1 month</li>
				<li class="date-picker__list__column-item">24 Hours</li>
			</ul>
			<ul class="date-picker__list__column">
				<li class="date-picker__list__column-item">6 Months</li>
				<li class="date-picker__list__column-item">1 Week</li>
				<li class="date-picker__list__column-item">1 Year</li>
			</ul>
		</div>
		<div></div>
	</div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component

export default class DatePicker extends Vue {
}
</script>

<style scoped lang="scss">
.date-picker {
	background: #FFF;
	width: 530px;
	height: 405px;
	border: 1px solid #384B65;
	padding: 34px;
	&__list {
		-moz-column-count: 2;
	   -moz-column-gap: 48px;
	   -webkit-column-count: 2;
	   column-count: 2;
	   border-bottom: 1px solid #AFB7C1;

	   &__column {
		   list-style-type: none;
		   padding-left: 0;
	   }

	   &__column-item {
			margin-bottom: 34px;
			font-style: normal;
			font-weight: 400;

		   &:hover {
				font-weight: bold;
				font-style: normal;
				font-weight: 700;
				background: #F5F6FA;
				border-left: 5px solid #2582FF;
		   }
	   }
	}
}
</style>
