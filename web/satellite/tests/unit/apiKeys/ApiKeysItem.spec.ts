// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import ApiKeysItem from '@/components/apiKeys/ApiKeysItem.vue';

import { mount } from '@vue/test-utils';

describe('ApiKeysItem.vue', (): void => {
    it('renders correctly', (): void => {
        const wrapper = mount(ApiKeysItem);

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with default props', (): void => {
        const wrapper = mount(ApiKeysItem);

        expect(wrapper.vm.$props.itemData).toEqual({ createdAt: '', id: '', isSelected: false, name: '', secret: '' });
    });
});
