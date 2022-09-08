// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount } from '@vue/test-utils';

import { SortDirection } from '@/types/common';
import { ProjectMemberOrderBy } from '@/types/projectMembers';

import SortingListHeader from '@/components/team/SortingListHeader.vue';

describe('SortingListHeader.vue', () => {
    it('should render correctly', function () {
        const wrapper = mount(SortingListHeader);

        expect(wrapper).toMatchSnapshot();
    });

    it('should retrieve callback', function () {
        const onPressSpy = jest.fn();

        const wrapper = mount(SortingListHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            },
        });
        wrapper.find('.sort-header-container__name-container').trigger('click');
        expect(onPressSpy).toHaveBeenCalledTimes(1);
    });

    it('should change sort direction', function () {
        const onPressSpy = jest.fn();

        const wrapper = mount(SortingListHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            },
        });

        expect(wrapper.vm.$data.sortBy).toBe(ProjectMemberOrderBy.NAME);
        expect(wrapper.vm.$data.sortDirection).toBe(SortDirection.ASCENDING);

        wrapper.find('.sort-header-container__name-container').trigger('click');
        expect(onPressSpy).toHaveBeenCalledTimes(1);

        expect(wrapper.vm.$data.sortBy).toBe(ProjectMemberOrderBy.NAME);
        expect(wrapper.vm.$data.sortDirection).toBe(SortDirection.DESCENDING);
    });

    it('should change sort by value', function () {
        const onPressSpy = jest.fn();

        const wrapper = mount(SortingListHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            },
        });

        expect(wrapper.vm.$data.sortBy).toBe(ProjectMemberOrderBy.NAME);
        expect(wrapper.vm.$data.sortDirection).toBe(SortDirection.ASCENDING);

        wrapper.find('.sort-header-container__added-container').trigger('click');
        expect(onPressSpy).toHaveBeenCalledTimes(1);

        expect(wrapper.vm.$data.sortBy).toBe(ProjectMemberOrderBy.CREATED_AT);
        expect(wrapper.vm.$data.sortDirection).toBe(SortDirection.ASCENDING);
    });
});
