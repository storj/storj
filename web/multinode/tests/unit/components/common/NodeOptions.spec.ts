// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import NodeOptions from '@/app/components/common/NodeOptions.vue';

import { shallowMount } from '@vue/test-utils';

describe('NodeOptions', (): void => {
    it('renders correctly', async (): Promise<void> => {
        const wrapper = shallowMount(NodeOptions, {
            propsData: { id: 'id' },
        });

        expect(wrapper).toMatchSnapshot();

        await wrapper.find('.options-button').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });
});
