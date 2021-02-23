// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import NoApiKeysArea from '@/components/apiKeys/NoApiKeysArea.vue';

import { shallowMount } from '@vue/test-utils';

describe('NoApiKeysArea.vue', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(NoApiKeysArea);

        expect(wrapper).toMatchSnapshot();
    });
});
