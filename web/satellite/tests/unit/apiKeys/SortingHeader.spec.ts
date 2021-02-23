// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';

import SortingHeader from '@/components/apiKeys/SortingHeader.vue';

import { ApiKeyOrderBy } from '@/types/apiKeys';
import { SortDirection } from '@/types/common';
import { mount } from '@vue/test-utils';

describe('SortingHeader.vue', (): void => {
    it('should render correctly', (): void => {
        const wrapper = mount(SortingHeader);

        expect(wrapper).toMatchSnapshot();
    });

    it('should retrieve callback', (): void => {
        const onPressSpy = sinon.spy();

        const wrapper = mount(SortingHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            },
        });
        wrapper.find('.sort-header-container__name-item').trigger('click');
        expect(onPressSpy.callCount).toBe(1);
    });

    it('should change sort direction', (): void => {
        const onPressSpy = sinon.spy();

        const wrapper = mount(SortingHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            },
        });

        expect(wrapper.vm.$data.sortBy).toBe(ApiKeyOrderBy.NAME);
        expect(wrapper.vm.$data.sortDirection).toBe(SortDirection.ASCENDING);

        wrapper.find('.sort-header-container__name-item').trigger('click');
        expect(onPressSpy.callCount).toBe(1);

        expect(wrapper.vm.$data.sortBy).toBe(ApiKeyOrderBy.NAME);
        expect(wrapper.vm.$data.sortDirection).toBe(SortDirection.DESCENDING);
    });

    it('should change sort by value', (): void => {
        const onPressSpy = sinon.spy();

        const wrapper = mount(SortingHeader, {
            propsData: {
                onHeaderClickCallback: onPressSpy,
            },
        });

        expect(wrapper.vm.$data.sortBy).toBe(ApiKeyOrderBy.NAME);
        expect(wrapper.vm.$data.sortDirection).toBe(SortDirection.ASCENDING);

        wrapper.find('.sort-header-container__date-item').trigger('click');
        expect(onPressSpy.callCount).toBe(1);

        expect(wrapper.vm.$data.sortBy).toBe(ApiKeyOrderBy.CREATED_AT);
        expect(wrapper.vm.$data.sortDirection).toBe(SortDirection.ASCENDING);
    });
});
