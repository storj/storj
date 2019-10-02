// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import VDatePicker from '@/components/common/VDatePicker.vue';

import { mount } from '@vue/test-utils';

const months = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];

describe('VDatePicker.vue', () => {
    it('renders correctly', function () {
        const wrapper = mount(VDatePicker, {});

        wrapper.vm.showCheck();

        expect(wrapper.findAll('li').at(0).text()).toBe('Mo');
        expect(wrapper.findAll('.day').length).toBe(42);

        wrapper.vm.showYear();

        expect(wrapper.findAll('.year').length).toBe(100);

        wrapper.vm.showMonth();

        expect(wrapper.findAll('.month').length).toBe(12);
    });

    it('renders correctly with props', function () {
        const wrapper = mount(VDatePicker, {
            propsData: {
                isSundayFirst: true,
            },
        });

        wrapper.vm.showCheck();

        expect(wrapper.findAll('li').at(0).text()).toBe('Su');
        expect(wrapper.findAll('.day').length).toBe(42);
    });

    it('triggers correct functionality on normal check', function () {
        const wrapper = mount(VDatePicker);

        wrapper.vm.showCheck();
        wrapper.vm.setYear(2019);
        wrapper.vm.setMonth(months[9]);

        wrapper.findAll('.day').at(1).trigger('click');

        expect(wrapper.vm.selectedDays.length).toBe(1);
        const selectedDay = wrapper.vm.selectedDays[0];
        expect(selectedDay.getDate()).toBe(1);
        expect(selectedDay.getMonth()).toBe(9);
        expect(selectedDay.getFullYear()).toBe(2019);

        wrapper.findAll('.day').at(2).trigger('click');

        expect(wrapper.vm.selectedDays.length).toBe(0);
    });

    it('triggers correct functionality on toggle checking', function () {
        const wrapper = mount(VDatePicker);

        wrapper.vm.showCheck();
        wrapper.vm.setYear(2019);
        wrapper.vm.setMonth(months[9]);

        wrapper.findAll('.day').at(1).trigger('click');

        expect(wrapper.vm.selectedDays.length).toBe(1);
        const selectedDay1 = wrapper.vm.selectedDays[0];
        expect(selectedDay1.getDate()).toBe(1);
        expect(selectedDay1.getMonth()).toBe(9);
        expect(selectedDay1.getFullYear()).toBe(2019);

        wrapper.findAll('.day').at(1).trigger('click');

        expect(wrapper.vm.selectedDays.length).toBe(0);

        wrapper.findAll('.day').at(2).trigger('click');

        expect(wrapper.vm.selectedDays.length).toBe(1);
        const selectedDay2 = wrapper.vm.selectedDays[0];
        expect(selectedDay2.getDate()).toBe(2);
        expect(selectedDay2.getMonth()).toBe(9);
        expect(selectedDay2.getFullYear()).toBe(2019);
    });

    it('triggers correct functionality on month selection', function () {
        const wrapper = mount(VDatePicker);

        wrapper.vm.showCheck();
        wrapper.vm.setYear(2019);
        wrapper.vm.setMonth(months[9]);

        expect(wrapper.findAll('.month').length).toBe(0);

        wrapper.find('.month-selection').trigger('click');

        expect(wrapper.findAll('.month').length).toBe(12);

        wrapper.findAll('.month').at(0).trigger('click');

        expect(wrapper.vm.selectedDateState.month).toBe(0);
        expect(wrapper.find('.month-selection').text()).toBe(months[0]);
    });

    it('triggers correct functionality on year selection', function () {
        const wrapper = mount(VDatePicker);
        const nowYear = new Date().getFullYear();

        wrapper.vm.showCheck();
        wrapper.vm.setYear(2019);
        wrapper.vm.setMonth(months[9]);

        expect(wrapper.findAll('.year').length).toBe(0);

        wrapper.find('.year-selection').trigger('click');

        expect(wrapper.findAll('.year').length).toBe(100);
        expect(wrapper.findAll('.year').at(0).text()).toBe(nowYear.toString());

        wrapper.findAll('.year').at(1).trigger('click');

        expect(wrapper.vm.selectedDateState.year).toBe(nowYear - 1);
        expect(wrapper.find('.year-selection').text()).toBe((nowYear - 1).toString());
    });

    it('triggers correct functionality on month incrementation', function () {
        const wrapper = mount(VDatePicker);
        const now = new Date();
        const nowYear = now.getFullYear();

        wrapper.vm.showCheck();
        wrapper.vm.setYear(nowYear);
        wrapper.vm.setMonth(months[now.getMonth() - 1]);

        const actualDates = wrapper.findAll('.day');

        actualDates.at(actualDates.length - 1).trigger('click');

        expect(wrapper.vm.selectedDateState.year).toBe(nowYear);
        expect(wrapper.vm.selectedDateState.month).toBe(now.getMonth());

        wrapper.find('.cov-date-next').trigger('click');

        expect(wrapper.vm.selectedDateState.year).toBe(nowYear);
        expect(wrapper.vm.selectedDateState.month).toBe(now.getMonth());

        wrapper.vm.setYear(nowYear - 1);
        wrapper.vm.setMonth(months[0]);

        const changedDates = wrapper.findAll('.day');

        changedDates.at(changedDates.length - 1).trigger('click');

        expect(wrapper.vm.selectedDateState.year).toBe(nowYear - 1);
        expect(wrapper.vm.selectedDateState.month).toBe(1);

        wrapper.find('.cov-date-next').trigger('click');

        expect(wrapper.vm.selectedDateState.year).toBe(nowYear - 1);
        expect(wrapper.vm.selectedDateState.month).toBe(2);
    });

    it('triggers correct functionality on month decrementation', function () {
        const wrapper = mount(VDatePicker);

        wrapper.vm.showCheck();
        wrapper.vm.setYear(2019);
        wrapper.vm.setMonth(months[9]);

        wrapper.findAll('.day').at(0).trigger('click');

        expect(wrapper.vm.selectedDateState.month).toBe(8);

        wrapper.find('.cov-date-previous').trigger('click');

        expect(wrapper.vm.selectedDateState.month).toBe(7);

        wrapper.vm.setMonth(months[0]);

        wrapper.find('.cov-date-previous').trigger('click');

        expect(wrapper.vm.selectedDateState.year).toBe(2018);
        expect(wrapper.vm.selectedDateState.month).toBe(11);

        wrapper.vm.setYear(2019);
        wrapper.vm.setMonth(months[0]);

        wrapper.findAll('.day').at(0).trigger('click');

        expect(wrapper.vm.selectedDateState.year).toBe(2018);
        expect(wrapper.vm.selectedDateState.month).toBe(11);
    });
});
