// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';

import SortingListHeader from '@/components/team/SortingListHeader.vue';

import { SortDirection } from '@/types/common';
import { ProjectMemberOrderBy } from '@/types/projectMembers';
import { mount } from '@vue/test-utils';

describe('SortingListHeader.vue', () => {
    it('should render correctly', function () {
        const wrapper = mount(SortingListHeader);

        expect(wrapper).toMatchSnapshot();
    });

    it('should retrieve callback', function () {
        const onPressSpy = sinon.spy();

        const wrapper = mount(SortingListHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            },
        });
        wrapper.find('.sort-header-container__name-container').trigger('click');
        expect(onPressSpy.callCount).toBe(1);
    });

    it('should change sort direction', function () {
        const onPressSpy = sinon.spy();

        const wrapper = mount(SortingListHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            },
        });

        expect(wrapper.vm.sortBy).toBe(ProjectMemberOrderBy.NAME);
        expect(wrapper.vm.sortDirection).toBe(SortDirection.ASCENDING);

        wrapper.find('.sort-header-container__name-container').trigger('click');
        expect(onPressSpy.callCount).toBe(1);

        expect(wrapper.vm.sortBy).toBe(ProjectMemberOrderBy.NAME);
        expect(wrapper.vm.sortDirection).toBe(SortDirection.DESCENDING);
    });

    it('should change sort by value', function () {
        const onPressSpy = sinon.spy();

        const wrapper = mount(SortingListHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            },
        });

        expect(wrapper.vm.sortBy).toBe(ProjectMemberOrderBy.NAME);
        expect(wrapper.vm.sortDirection).toBe(SortDirection.ASCENDING);

        wrapper.find('.sort-header-container__added-container').trigger('click');
        expect(onPressSpy.callCount).toBe(1);

        expect(wrapper.vm.sortBy).toBe(ProjectMemberOrderBy.CREATED_AT);
        expect(wrapper.vm.sortDirection).toBe(SortDirection.ASCENDING);
    });
});
