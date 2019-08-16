// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount } from '@vue/test-utils';
import sinon from 'sinon';
import SortingListHeader from '@/components/team/SortingListHeader.vue';
import { ProjectMemberSortByEnum, ProjectMemberSortDirectionEnum } from '@/utils/constants/ProjectMemberSortEnum';

describe('SortingListHeader.vue', () => {
    it('should render correctly', function () {
        const wrapper = mount(SortingListHeader);

        expect(wrapper).toMatchSnapshot();
    });

    it('should retrieve callback', function () {
        let onPressSpy = sinon.spy();

        const wrapper = mount(SortingListHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            }
        });
        wrapper.find('.sort-header-container__name-container').trigger('click');
        expect(onPressSpy.callCount).toBe(1);
    });

    it('should change sort direction', function () {
        let onPressSpy = sinon.spy();

        const wrapper = mount(SortingListHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            }
        });

        expect(wrapper.vm.sortBy).toBe(ProjectMemberSortByEnum.NAME);
        expect(wrapper.vm.sortDirection).toBe(ProjectMemberSortDirectionEnum.ASCENDING);

        wrapper.find('.sort-header-container__name-container').trigger('click');
        expect(onPressSpy.callCount).toBe(1);

        expect(wrapper.vm.sortBy).toBe(ProjectMemberSortByEnum.NAME);
        expect(wrapper.vm.sortDirection).toBe(ProjectMemberSortDirectionEnum.DESCENDING);
    });

    it('should change sort by value', function () {
        let onPressSpy = sinon.spy();

        const wrapper = mount(SortingListHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            }
        });

        expect(wrapper.vm.sortBy).toBe(ProjectMemberSortByEnum.NAME);
        expect(wrapper.vm.sortDirection).toBe(ProjectMemberSortDirectionEnum.ASCENDING);

        wrapper.find('.sort-header-container__added-container').trigger('click');
        expect(onPressSpy.callCount).toBe(1);

        expect(wrapper.vm.sortBy).toBe(ProjectMemberSortByEnum.CREATED_AT);
        expect(wrapper.vm.sortDirection).toBe(ProjectMemberSortDirectionEnum.ASCENDING);
    });
});
