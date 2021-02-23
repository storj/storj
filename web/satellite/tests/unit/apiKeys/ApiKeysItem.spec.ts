// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import ApiKeysItem from '@/components/apiKeys/ApiKeysItem.vue';

import { ApiKey } from '@/types/apiKeys';
import { mount } from '@vue/test-utils';

describe('ApiKeysItem.vue', (): void => {
    it('renders correctly', (): void => {
        const wrapper = mount(ApiKeysItem, {
            propsData: {
                itemData: new ApiKey('', '', new Date(0), ''),
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
