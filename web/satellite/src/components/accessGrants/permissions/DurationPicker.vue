// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="duration-picker">
        <div class="duration-picker__list">
            <ul class="duration-picker__list__column">
                <li v-on:click="onDurationClick" class="duration-picker__list__column-item">Forever</li>
                <li v-on:click="onDurationClick" class="duration-picker__list__column-item">1 month</li>
                <li v-on:click="onDurationClick" class="duration-picker__list__column-item">24 Hours</li>
            </ul>
            <ul class="duration-picker__list__column">
                <li v-on:click="onDurationClick" class="duration-picker__list__column-item">6 Months</li>
                <li v-on:click="onDurationClick" class="duration-picker__list__column-item">1 Week</li>
                <li v-on:click="onDurationClick" class="duration-picker__list__column-item">1 Year</li>
            </ul>
        </div>
        <hr class="duration-picker__break">
        <div class="duration-picker__date-picker__wrapper">
            <DatePicker
                range
                open
                :append-to-body="false"
                :inline="true"
                v-model="date"
                popup-class="duration-picker__date-picker__popup"
                input-class="duration-picker__date-picker__input"
                @change="onDateChange($event)"
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
    private duration: string = '';
    private dateRange: string[] = [];

    /**
     * When date range value changes
     * @param dateRange
     */
    public onDateChange(date): void {
        this.dateRange = date;
    }

    /**
     * When duration button is clicked
     * @param duration
     */
    public onDurationClick(event): void {
        this.duration = event.target.textContent;
    }
}
</script>

<style scoped lang="scss">
    .duration-picker {
        background: #fff;
        width: 530px;
        height: 400px;
        border: 1px solid #384b65;
        margin: 0 auto;
        -webkit-box-shadow: 0 4px 8px 0 rgba(0, 0, 0, 0.2), 0 6px 20px 0 rgba(0, 0, 0, 0.19);
        -moz-box-shadow: 0 4px 8px 0 rgba(0, 0, 0, 0.2), 0 6px 20px 0 rgba(0, 0, 0, 0.19);
        box-shadow: 0 4px 8px 0 rgba(0, 0, 0, 0.2), 0 6px 20px 0 rgba(0, 0, 0, 0.19);
        position: absolute;
        z-index: 1;
        right: 0;
        top: 100%;
        &__list {
            -moz-column-count: 2;
            -moz-column-gap: 48px;
            -webkit-column-count: 2;
            column-count: 2;
            padding: 10px 24px 0;
            &__column {
                list-style-type: none;
                padding-left: 0;
                margin-top: 0;
            }
            &__column-item {
                font-size: 14px;
                font-weight: 400;
                padding: 10px 0 10px 12px;
                border-left: 7px solid #fff;
                color: #1b2533;
                &:hover {
                    font-weight: bold;
                    background: #f5f6fa;
                    border-left: 6px solid #2582ff;
                    cursor: pointer;
                }
            }
        }
        &__break {
            width: 84%;
            margin: 0 auto;
        }
        &__date-picker {
            &__wrapper {
                position: relative;
                margin: 0;
            }
        }
    }
</style>

<style lang="scss">
    .duration-picker {
        &__date-picker {
            &__popup {
                position: absolute;
                top: 0;
                left: 25px;
                width: 480px;
                height: 210px;
            }
        }
    }

    .mx-calendar {
        height: 210px;
    }

    .mx-table-date td, .mx-table-date th {
        height: 12px;
        font-size: 10px;
    }

    .mx-table {
        height: 70%;
    }
</style>
