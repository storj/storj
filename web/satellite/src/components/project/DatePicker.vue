// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
	<div class="cov-vue-date">
		<div class="datepickbox">
			<input type="text" title="input date" class="cov-datepicker" readonly="readonly" :placeholder="option.placeholder" v-model="date.time" :required="required" :style="option.inputStyle ? option.inputStyle : {}" />
		</div>
		<div class="datepicker-overlay" v-if="showInfo.check" @click="dismiss($event)" :style="{'background' : option.overlayOpacity? 'rgba(0,0,0,'+option.overlayOpacity+')' : 'rgba(0,0,0,0.5)'}">
			<div class="cov-date-body" :style="{'background-color': option.color ? option.color.header : '#3f51b5'}">
				<div class="cov-date-monthly">
					<div class="cov-date-previous" @click="nextMonth('pre')">«</div>
					<div class="cov-date-caption" :style="{'color': option.color ? option.color.headerText : '#fff'}">
						<span @click="showYear">{{checked.year}}</span>
						<span @click="showMonth">{{displayInfo.month}}</span>
					</div>
					<div class="cov-date-next" @click="nextMonth('next')">»</div>
				</div>
				<div class="cov-date-box" v-if="showInfo.day">
					<div class="cov-picker-box">
						<div class="week">
							<ul>
								<li v-for="week in weekLibrary" :key="week">{{week}}</li>
							</ul>
						</div>
						<div class="day" v-for="(day, index) in dayList" :key="index" @click="checkDay(day)" :class="{'checked':day.checked,'unavailable':day.unavailable,'passive-day': !(day.inMonth), 'today': day.today}" :style="day.checked ? (option.color && option.color.checkedDay ? { background: option.color.checkedDay } : { background: '#2683FF' }) : {}">{{day.value}}</div>
					</div>
				</div>
				<div class="cov-date-box list-box" v-if="showInfo.year">
					<div class="cov-picker-box date-list" id="yearList">
						<div class="date-item" v-for="yearItem in yearLibrary" :key="yearItem" @click="setYear(yearItem)">{{yearItem}}</div>
					</div>
				</div>
				<div class="cov-date-box list-box" v-if="showInfo.month">
					<div class="cov-picker-box date-list">
						<div class="date-item" v-for="monthItem in monthLibrary" :key="monthItem" @click="setMonth(monthItem)">{{monthItem}}</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { DateObj, Options } from '@/types/datepicker';

@Component
export default class DatePicker extends Vue {

	@Prop({default: false})
	private required: boolean;
	@Prop({default: () => new DateObj()})
	private date: DateObj;
	@Prop({default: () => new Options()})
	private option: Options;

	public showInfo: any = {
		day: false,
		month: false,
		year: false,
		check: false,
	};
	public displayInfo = {
		month: '',
	};

	public yearLibrary = [];
	public weekLibrary = this.option.week;
	public monthLibrary = this.option.month;
	public checked: any = {
		currentMoment: null,
		year: 0,
		month: 0,
		day: 0,
	};
	public dayList = [];
	public selectedDays: Date[] = [];

	public nextMonth(type) {
		const currentMoment = new Date(this.checked.year, this.checked.month, this.checked.day);
		const currentMonth = currentMoment.getMonth();
		const now = new Date();

		if (type === 'next') {
			if (currentMonth === now.getMonth() && currentMoment.getFullYear() === now.getFullYear()) {
				return;
			}
			currentMoment.setMonth(currentMonth + 1);
		} else {
			currentMoment.setMonth(currentMonth - 1);
		}
		this.showDay(currentMoment);
	}

	public checkDay(obj) {
		if (obj.unavailable || obj.value === '') {
			return false;
		}
		if (!obj.inMonth) {
			this.nextMonth(obj.action);

			return;
		}

		if (obj.checked) {
			obj.checked = false;
			this.selectedDays.splice(this.selectedDays.indexOf(obj.moment), 1);

			return;
		}

		if (this.selectedDays.length < 2) {
			this.selectedDays.push(obj.moment);
			obj.checked = true;
		}

		if (this.selectedDays.length === 2) {
			this.submitSelectedDays();
		}
	}

	public setYear(year) {
		this.checked.currentMoment = new Date(year, this.checked.month, this.checked.day);
		this.showDay(this.checked.currentMoment);
	}

	public setMonth(month) {
		const mo = this.monthLibrary.indexOf(month);

		this.checked.currentMoment = new Date(this.checked.year, mo, this.checked.day);
		this.showDay(this.checked.currentMoment);
	}

	public dismiss(evt) {
		if (evt.target.className !== 'datepicker-overlay') {
			return;
		}
		if (!this.option.dismissible) {
			return;
		}

		this.selectedDays = [];

		this.showInfo.check = false;
		this.$emit('cancel');
	}

	public showCheck() {
		this.showDay();
		this.showInfo.check = true;
	}

	public showYear() {
		const year = new Date().getFullYear();
		this.yearLibrary = [];
		const yearTmp = [];
		for (let i = year - 100; i <= year; i++) {
			yearTmp.unshift(i);
		}
		this.yearLibrary = yearTmp;
		this.showInfo.day = false;
		this.showInfo.year = true;
		this.showInfo.month = false;
	}

	public showMonth(): void {
		this.showInfo.day = false;
		this.showInfo.year = false;
		this.showInfo.month = true;
	}

	private submitSelectedDays(): void {
		this.$emit('change', this.selectedDays);
		this.showInfo.check = false;
		this.selectedDays = [];
	}

	private showDay(time: Date = new Date()) {
		this.checked.currentMoment = new Date(time.getFullYear(), time.getMonth(), time.getDate());
		this.showDays();
		this.checked.year = this.checked.currentMoment.getFullYear();
		this.checked.month = this.checked.currentMoment.getMonth();
		this.checked.day = this.checked.currentMoment.getDate();
		this.displayInfo.month = this.monthLibrary[this.checked.month];
		const days = [];
		const firstDate = new Date(this.checked.year, this.checked.month, this.checked.day);
		firstDate.setDate(1);
		let firstDay = firstDate.getDay();
		const previousMonth = new Date(this.checked.year, this.checked.month, this.checked.day);
		const nextMonth = new Date(this.checked.year, this.checked.month, this.checked.day);
		nextMonth.setMonth(nextMonth.getMonth() + 1);
		previousMonth.setMonth(previousMonth.getMonth() - 1);
		const monthDays = new Date(this.checked.year, parseInt(this.checked.month) + 1, 0).getDate();
		const now = new Date();
		const nowMonth = now.getMonth();
		for (let i = 1; i <= monthDays; i++) {
			const moment = new Date(this.checked.year, this.checked.month, this.checked.day);
			moment.setDate(i);
			days.push({
				value: i,
				inMonth: this.checked.month !== nowMonth || (this.checked.month === nowMonth && i <= now.getDate()),
				unavailable: false,
				checked: false,
				moment,
			});
		}
		if (firstDay === 0) firstDay = 7;
		const daysInPreviousMonth = new Date(previousMonth.getFullYear(), previousMonth.getMonth() + 1, 0).getDate();
		for (let _i = 0; _i < firstDay - (this.option.isSundayFirst ? 0 : 1); _i++) {
			const moment = new Date(this.checked.year, this.checked.month, this.checked.day);
			moment.setDate(1);
			moment.setMonth(moment.getMonth() - 1);
			moment.setDate(new Date(moment.getFullYear(), moment.getMonth() + 1, 0).getDate() - _i);
			const passiveDay = {
				value: daysInPreviousMonth - _i,
				inMonth: false,
				action: 'previous',
				unavailable: false,
				checked: false,
				moment,
			};
			days.unshift(passiveDay);
		}
		const passiveDaysAtFinal = 42 - days.length;
		for (let _i2 = 1; _i2 <= passiveDaysAtFinal; _i2++) {
			const moment = new Date(this.checked.year, this.checked.month, this.checked.day);
			moment.setMonth(moment.getMonth() + 1);
			moment.setDate(_i2);
			const _passiveDay = {
				value: _i2,
				inMonth: false,
				action: 'next',
				unavailable: false,
				checked: false,
				moment,
			};
			days.push(_passiveDay);
		}
		this.markToday(days);
		this.dayList = days;
	}

	private markToday(days) {
		const now = new Date();
		const yearNow = now.getFullYear();
		const monthNow = now.getMonth();
		const dateNow = now.getDate();

		days.forEach((day: any) => {
			if (day.moment.getFullYear() === yearNow && day.moment.getMonth() === monthNow && day.moment.getDate() === dateNow) {
				day.today = true;
			}
		});
	}

	private showDays(): void {
		this.showInfo.day = true;
		this.showInfo.year = false;
		this.showInfo.month = false;
	}
}
</script>

<style scoped lang="scss">
	.datepicker-overlay {
		position: fixed;
		width: 100%;
		height: 100%;
		z-index: 998;
		top: 0;
		left: 0;
		overflow: hidden;
		-webkit-animation: fadein 0.5s;
		/* Safari, Chrome and Opera > 12.1 */
		-moz-animation: fadein 0.5s;
		/* Firefox < 16 */
		-ms-animation: fadein 0.5s;
		/* Internet Explorer */
		-o-animation: fadein 0.5s;
		/* Opera < 12.1 */
		animation: fadein 0.5s;
	}
	@keyframes fadein {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}
	/* Firefox < 16 */
	@-moz-keyframes fadein {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}
	/* Safari, Chrome and Opera > 12.1 */
	@-webkit-keyframes fadein {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}
	/* Internet Explorer */
	@-ms-keyframes fadein {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}
	/* Opera < 12.1 */
	@-o-keyframes fadein {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}
	.cov-date-body {
		background: #3F51B5;
		overflow: hidden;
		font-size: 16px;
		font-weight: 400;
		position: fixed;
		display: block;
		width: 400px;
		max-width: 100%;
		z-index: 999;
		top: 50%;
		left: 50%;
		-webkit-transform: translate(-50%, -50%);
		-ms-transform: translate(-50%, -50%);
		transform: translate(-50%, -50%);
		box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.2);
		font-family: 'font_medium';
	}
	.cov-picker-box {
		background: #fff;
		display: inline-block;
		padding: 25px;
		box-sizing: border-box !important;
		-moz-box-sizing: border-box !important;
		-webkit-box-sizing: border-box !important;
		-ms-box-sizing: border-box !important;
		width: 400px;
		max-width: 100%;
		height: 280px;
		text-align: start!important;
	}
	.cov-picker-box td {
		height: 34px;
		width: 34px;
		padding: 0;
		line-height: 34px;
		color: #000;
		background: #fff;
		text-align: center;
		cursor: pointer;
	}
	.cov-picker-box td:hover {
		background: #E6E6E6;
	}
	table {
		border-collapse: collapse;
		border-spacing: 0;
		width: 100%;
	}
	.day {
		width: 14.2857143%;
		display: inline-block;
		text-align: center;
		cursor: pointer;
		height: 34px;
		padding: 0;
		line-height: 34px;
		color: #000;
		background: #fff;
		vertical-align: middle;
	}
	.week ul {
		margin: 0 0 8px;
		padding: 0;
		list-style: none;
	}
	.week ul li {
		width: 14.2%;
		display: inline-block;
		text-align: center;
		background: transparent;
		color: #000;
		font-weight: bold;
	}
	.passive-day {
		color: #bbb;
	}
	.checked {
		background: #2683FF;
		color: #FFF !important;
	}
	.unavailable {
		color: #ccc;
		cursor: not-allowed;
	}
	.cov-date-monthly {
		height: 50px;
	}
	.cov-date-monthly > div {
		display: inline-block;
		padding: 0;
		margin: 0;
		vertical-align: middle;
		color: #fff;
		height: 50px;
		float: left;
		text-align: center;
	}
	.cov-date-previous,
	.cov-date-next {
		position: relative;
		width: 20% !important;
		text-indent: -300px;
		overflow: hidden;
		color: #fff;
	}
	.cov-date-caption {
		width: 60%;
		padding: 10px 0!important;
		box-sizing: border-box;
		font-size: 18px;
		font-family: 'font_medium';
		line-height: 30px;
		display: flex;
		flex-direction: column;
		justify-content: center;
		align-items: center;

		span {
			padding: 0 3px;
		}
	}
	.cov-date-previous:hover,
	.cov-date-next:hover {
		background: rgba(255, 255, 255, 0.1);
	}
	.day:hover {
		background: #EAEAEA;
	}
	.unavailable:hover {
		background: none;
	}
	.cov-date-next::before,
	.cov-date-previous::before {
		width: 20px;
		height: 2px;
		text-align: center;
		position: absolute;
		background: #fff;
		top: 50%;
		margin-top: -7px;
		margin-left: -7px;
		left: 50%;
		line-height: 0;
		content: '';
		-webkit-transform: rotate(45deg);
		-moz-transform: rotate(45deg);
		transform: rotate(45deg);
	}
	.cov-date-next::after,
	.cov-date-previous::after {
		width: 20px;
		height: 2px;
		text-align: center;
		position: absolute;
		background: #fff;
		margin-top: 6px;
		margin-left: -7px;
		top: 50%;
		left: 50%;
		line-height: 0;
		content: '';
		-webkit-transform: rotate(-45deg);
		-moz-transform: rotate(-45deg);
		transform: rotate(-45deg);
	}
	.cov-date-previous::after {
		-webkit-transform: rotate(45deg);
		-moz-transform: rotate(45deg);
		transform: rotate(45deg);
	}
	.cov-date-previous::before {
		-webkit-transform: rotate(-45deg);
		-moz-transform: rotate(-45deg);
		transform: rotate(-45deg);
	}
	.date-item {
		text-align: center;
		font-size: 20px;
		padding: 10px 0;
		cursor: pointer;
	}
	.date-item:hover {
		background: #e0e0e0;
	}
	.date-list {
		overflow: auto;
		vertical-align: top;
		padding: 0;
	}
	.cov-vue-date {
		display: inline-block;
		color: #5D5D5D;
	}
	.button-box {
		background: #fff;
		vertical-align: top;
		height: 50px;
		line-height: 50px;
		text-align: right;
		padding-right: 20px;
	}
	.button-box span {
		cursor: pointer;
		padding: 10px 20px;
	}
	.watch-box {
		height: 100%;
		overflow: hidden;
	}
	::-webkit-scrollbar {
		width: 2px;
	}
	::-webkit-scrollbar-track {
		background: #E3E3E3;
	}
	::-webkit-scrollbar-thumb {
		background: #C1C1C1;
		border-radius: 2px;
	}
	.cov-date-box {
		font-family: 'font_medium';
	}
	.today {
		background: red;
		color: white;
	}
</style>
