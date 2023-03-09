// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount } from '@vue/test-utils';

import { SortingDirectionEnum } from '@/types/sortingArrows';

import VerticalArrows from '@/components/common/VerticalArrows.vue';

describe('VerticalArrows.vue', () => {
    it('should render with bottom arrow highlighted', function () {
        const wrapper = mount(VerticalArrows, {
            propsData: {
                isActive: true,
                direction: SortingDirectionEnum.BOTTOM,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('should render without any highlighted arrows', function () {
        const wrapper = mount(VerticalArrows, {
            propsData: {
                isActive: false,
                direction: SortingDirectionEnum.BOTTOM,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('should render with top arrow highlighted', function () {
        const wrapper = mount(VerticalArrows, {
            propsData: {
                isActive: true,
                direction: SortingDirectionEnum.TOP,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
