// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { shallowMount } from '@vue/test-utils';

import { Page } from '@/types/pagination';

import PagesBlock from '@/components/common/PagesBlock.vue';

describe('Pagination.vue', () => {
    it('renders correctly without props', () => {
        const wrapper = shallowMount(PagesBlock);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with props', () => {
        const callbackSpy = jest.fn();
        const pagesArray: Page[] = [];
        const SELECTED_PAGE_INDEX = 3;

        for (let i = 1; i <= 4; i++) {
            pagesArray.push(new Page(i, callbackSpy));
        }

        const wrapper = shallowMount(PagesBlock, {
            propsData: {
                pages: pagesArray,
                isSelected: (i: number) => i === SELECTED_PAGE_INDEX,
            },
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.findAll('span').length).toBe(4);
        expect(wrapper.findAll('span').at(2)?.classes().includes('selected')).toBe(true);
    });

    it('behaves correctly on page click', () => {
        const callbackSpy = jest.fn();
        const pagesArray: Page[] = [];

        for (let i = 1; i <= 3; i++) {
            pagesArray.push(new Page(i, callbackSpy));
        }

        const wrapper = shallowMount(PagesBlock, {
            propsData: {
                pages: pagesArray,
                isSelected: () => false,
            },
        });

        wrapper.findAll('span').at(1)?.trigger('click');

        expect(callbackSpy).toHaveBeenCalledTimes(1);
    });
});
