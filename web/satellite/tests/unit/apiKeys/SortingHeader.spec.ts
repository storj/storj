// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';

import SortingHeader from '@/components/apiKeys/SortingHeader.vue';

import { ApiKeyOrderBy } from '@/types/apiKeys';
import { SortDirection } from '@/types/common';
import { mount } from '@vue/test-utils';

describe('SortingHeader.vue', () => {
    it('should render correctly', function () {
        const wrapper = mount(SortingHeader);

        expect(wrapper).toMatchSnapshot();
    });

    it('should retrieve callback', function () {
        const onPressSpy = sinon.spy();

        const wrapper = mount(SortingHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            }
        });
        wrapper.find('.sort-header-container__name-item').trigger('click');
        expect(onPressSpy.callCount).toBe(1);
    });

    it('should change sort direction', function () {
        const onPressSpy = sinon.spy();

        const wrapper = mount(SortingHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            }
        });

        expect(wrapper.vm.sortBy).toBe(ApiKeyOrderBy.NAME);
        expect(wrapper.vm.sortDirection).toBe(SortDirection.ASCENDING);

        wrapper.find('.sort-header-container__name-item').trigger('click');
        expect(onPressSpy.callCount).toBe(1);

        expect(wrapper.vm.sortBy).toBe(ApiKeyOrderBy.NAME);
        expect(wrapper.vm.sortDirection).toBe(SortDirection.DESCENDING);
    });

    it('should change sort by value', function () {
        const onPressSpy = sinon.spy();

        const wrapper = mount(SortingHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            }
        });

        expect(wrapper.vm.sortBy).toBe(ApiKeyOrderBy.NAME);
        expect(wrapper.vm.sortDirection).toBe(SortDirection.ASCENDING);

        wrapper.find('.sort-header-container__date-item').trigger('click');
        expect(onPressSpy.callCount).toBe(1);

        expect(wrapper.vm.sortBy).toBe(ApiKeyOrderBy.CREATED_AT);
        expect(wrapper.vm.sortDirection).toBe(SortDirection.ASCENDING);
    });
});
