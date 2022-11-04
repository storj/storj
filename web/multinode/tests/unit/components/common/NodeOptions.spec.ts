// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { shallowMount } from '@vue/test-utils';

import NodeOptions from '@/app/components/common/NodeOptions.vue';

describe('NodeOptions', (): void => {
    it('renders correctly', async(): Promise<void> => {
        const wrapper = shallowMount(NodeOptions, {
            propsData: { id: 'id' },
            directives: {
                clickOutside: {},
            },
        });

        expect(wrapper).toMatchSnapshot();

        await wrapper.find('.options-button').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });
});
