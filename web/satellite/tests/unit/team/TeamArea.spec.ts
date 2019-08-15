// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount, shallowMount } from '@vue/test-utils';
import TeamArea from '@/components/team/TeamArea.vue';

describe('TeamArea.vue', () => {
    it('renders correctly', () => {
        const wrapper = mount(TeamArea);

        expect(wrapper).toMatchSnapshot();
    });
});
