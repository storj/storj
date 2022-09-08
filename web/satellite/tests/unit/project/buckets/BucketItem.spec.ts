// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, shallowMount } from '@vue/test-utils';

import { Bucket } from '@/types/buckets';

import BucketItem from '@/components/project/buckets/BucketItem.vue';

const localVue = createLocalVue();

const bucket = new Bucket('name', 1, 1, 1, 1, new Date(), new Date());

describe('BucketItem.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(BucketItem, {
            localVue,
            propsData: {
                itemData: bucket,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
