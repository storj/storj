// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import * as sinon from 'sinon';

import PagesBlock from '@/components/common/PagesBlock.vue';

import { Page } from '@/types/pagination';
import { mount, shallowMount } from '@vue/test-utils';

describe('Pagination.vue', () => {
    it('renders correctly without props', () => {
        const wrapper = shallowMount(PagesBlock);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with props', () => {
        const callbackSpy = sinon.spy();
        const pagesArray: Page[] = [];
        const SELECTED_PAGE_INDEX: number = 3;

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
        expect(wrapper.findAll('span').at(2).classes().includes('selected')).toBe(true);
    });

    it('behaves correctly on page click', () => {
        const callbackSpy = sinon.spy();
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

        wrapper.findAll('span').at(1).trigger('click');

        expect(callbackSpy.callCount).toBe(1);
    });
});
