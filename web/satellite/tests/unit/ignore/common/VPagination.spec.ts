// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount, shallowMount } from '@vue/test-utils';

import Pagination from '@/components/common/VPagination.vue';

describe('Pagination.vue', () => {
    it('renders correctly', () => {
        const wrapper = shallowMount(Pagination);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with props', () => {
        const wrapper = shallowMount(Pagination, {
            propsData: {
                totalPageCount: 10,
                onPageClickCallback: () => new Promise(() => false),
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('inits correctly with totalPageCount equals 10 and current pageNumber in first block', async () => {
        const wrapper = mount(Pagination, {
            propsData: {
                totalPageCount: 10,
                onPageClickCallback: () => new Promise(() => false),
            },
        });

        const wrapperData = await wrapper.vm.$data;

        expect(wrapperData.currentPageNumber).toBe(1);
        expect(wrapperData.pagesArray.length).toBe(10);
        expect(wrapperData.firstBlockPages.length).toBe(3);
        expect(wrapperData.middleBlockPages.length).toBe(0);
        expect(wrapperData.lastBlockPages.length).toBe(1);
        expect(wrapper.findAll('span').at(0)?.classes().includes('selected')).toBe(true);
    });

    it('inits correctly with totalPageCount equals 4', () => {
        const wrapper = shallowMount(Pagination, {
            propsData: {
                totalPageCount: 4,
                onPageClickCallback: () => new Promise(() => false),
            },
        });

        const wrapperData = wrapper.vm.$data;

        expect(wrapperData.currentPageNumber).toBe(1);
        expect(wrapperData.pagesArray.length).toBe(4);
        expect(wrapperData.firstBlockPages.length).toBe(4);
        expect(wrapperData.middleBlockPages.length).toBe(0);
        expect(wrapperData.lastBlockPages.length).toBe(0);
    });

    it('behaves correctly on page click', async () => {
        const callbackSpy = jest.fn();

        const wrapper = mount(Pagination, {
            propsData: {
                totalPageCount: 9,
                onPageClickCallback: callbackSpy,
            },
        });

        const wrapperData = await wrapper.vm.$data;

        await wrapper.findAll('span').at(2)?.trigger('click');
        await expect(callbackSpy).toHaveBeenCalledTimes(1);

        expect(wrapperData.currentPageNumber).toBe(3);
        expect(wrapperData.firstBlockPages.length).toBe(1);
        expect(wrapperData.middleBlockPages.length).toBe(3);
        expect(wrapperData.lastBlockPages.length).toBe(1);
    });

    it('behaves correctly on next page button click', async () => {
        const callbackSpy = jest.fn();

        const wrapper = mount(Pagination, {
            propsData: {
                totalPageCount: 9,
                onPageClickCallback: callbackSpy,
            },
        });

        const wrapperData = wrapper.vm.$data;

        wrapper.findAll('.pagination-container__button').at(1)?.trigger('click');
        await expect(callbackSpy).toHaveBeenCalledTimes(1);

        expect(wrapperData.currentPageNumber).toBe(2);
        expect(wrapperData.firstBlockPages.length).toBe(3);
        expect(wrapperData.middleBlockPages.length).toBe(0);
        expect(wrapperData.lastBlockPages.length).toBe(1);
    });

    it('behaves correctly on previous page button click', async () => {
        const callbackSpy = jest.fn();

        const wrapper = mount(Pagination, {
            propsData: {
                totalPageCount: 9,
                onPageClickCallback: callbackSpy,
            },
        });

        await wrapper.vm.onPageClick(8);

        const wrapperData = wrapper.vm.$data;

        wrapper.findAll('.pagination-container__button').at(0)?.trigger('click');
        await expect(callbackSpy).toHaveBeenCalledTimes(2);

        expect(wrapperData.currentPageNumber).toBe(7);
        expect(wrapperData.firstBlockPages.length).toBe(1);
        expect(wrapperData.middleBlockPages.length).toBe(3);
        expect(wrapperData.lastBlockPages.length).toBe(1);
    });

    it('behaves correctly on previous page button click when current is 1', async () => {
        const callbackSpy = jest.fn();

        const wrapper = mount(Pagination, {
            propsData: {
                totalPageCount: 9,
                onPageClickCallback: callbackSpy,
            },
        });

        const wrapperData = wrapper.vm.$data;

        wrapper.findAll('.pagination-container__button').at(0)?.trigger('click');
        await expect(callbackSpy).toHaveBeenCalledTimes(0);

        expect(wrapperData.currentPageNumber).toBe(1);
        expect(wrapperData.firstBlockPages.length).toBe(3);
        expect(wrapperData.middleBlockPages.length).toBe(0);
        expect(wrapperData.lastBlockPages.length).toBe(1);
    });

    it('behaves correctly on next page button click when current is last', async () => {
        const callbackSpy = jest.fn();

        const wrapper = mount(Pagination, {
            propsData: {
                totalPageCount: 9,
                onPageClickCallback: callbackSpy,
            },
        });

        const wrapperData = wrapper.vm.$data;

        await wrapper.vm.onPageClick(9);

        wrapper.findAll('.pagination-container__button').at(1)?.trigger('click');
        await expect(callbackSpy).toHaveBeenCalledTimes(1);

        expect(wrapperData.currentPageNumber).toBe(9);
        expect(wrapperData.firstBlockPages.length).toBe(1);
        expect(wrapperData.middleBlockPages.length).toBe(0);
        expect(wrapperData.lastBlockPages.length).toBe(3);
    });

    it('should reset current page index to 1', async () => {
        const wrapper = shallowMount(Pagination, {
            propsData: {
                totalPageCount: 4,
                onPageClickCallback: () => Promise.resolve({}),
            },
        });

        await wrapper.vm.nextPage();

        expect(wrapper.vm.$data.currentPageNumber).toBe(2);

        wrapper.vm.resetPageIndex();

        const wrapperData = wrapper.vm.$data;

        expect(wrapperData.currentPageNumber).toBe(1);
        expect(wrapperData.pagesArray.length).toBe(4);
    });

    it('should completely reinitialize Pagination on totalPageCount change', async () => {
        const wrapper = shallowMount(Pagination, {
            propsData: {
                totalPageCount: 4,
                onPageClickCallback: () => Promise.resolve({}),
            },
        });

        await wrapper.vm.nextPage();

        expect(wrapper.vm.$data.currentPageNumber).toBe(2);

        await wrapper.setProps({ totalPageCount: 7 });

        const wrapperData = wrapper.vm.$data;

        expect(wrapperData.currentPageNumber).toBe(1);
        expect(wrapperData.pagesArray.length).toBe(7);
    });
});
