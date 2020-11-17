// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div class="duration-picker">
		<div class="duration-picker__list">
			<ul class="duration-picker__list__column">
				<li class="duration-picker__list__column-item">Forever</li>
				<li class="duration-picker__list__column-item">1 month</li>
				<li class="duration-picker__list__column-item">24 Hours</li>
			</ul>
			<ul class="duration-picker__list__column">
				<li class="duration-picker__list__column-item">6 Months</li>
				<li class="duration-picker__list__column-item">1 Week</li>
				<li class="duration-picker__list__column-item">1 Year</li>
			</ul>
		</div>
		<hr class="duration-picker__break">
		<div class="duration-picker__date-picker__wrapper">
			<DatePicker
				range
				open
				popup-class="duration-picker__date-picker__popup"
			/>
		</div>
	</div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import DatePicker from 'vue2-datepicker';
import 'vue2-datepicker/index.css';

@Component({
    components: {
        DatePicker,
    },
})

export default class DurationPicker extends Vue {
}
</script>

<style scoped lang="scss">
.duration-picker {
    background: #fff;
    width: 530px;
    height: 650px;
    border: 1px solid #384b65;

    &__list {
        -moz-column-count: 2;
        -moz-column-gap: 48px;
        -webkit-column-count: 2;
        column-count: 2;
        padding: 24px 24px 0;

        &__column {
            list-style-type: none;
            padding-left: 0;
            margin-top: 0;
        }

        &__column-item {
            margin-bottom: 24px;
            font-style: normal;
            font-weight: 400;
            padding: 10px 0 10px 12px;
            border-left: 7px solid #fff;
            color: #1b2533;

            &:hover {
                font-weight: bold;
                background: #f5f6fa;
                border-left: 7px solid #2582ff;
                cursor: pointer;
            }
        }
    }

    &__break {
        width: 82%;
    }

    &__date-picker {

        &__wrapper {
            text-align: center;
            margin: 40px auto;
        }

        &__popup {
            top: 521px;
            left: 267px;
        }
    }
}
</style>
