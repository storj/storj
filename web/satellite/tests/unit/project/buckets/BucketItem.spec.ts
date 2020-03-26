// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import BucketItem from '@/components/project/buckets/BucketItem.vue';

import { Bucket } from '@/types/buckets';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();

const bucket = new Bucket('name', 1, 1, 1, new Date(), new Date());

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
