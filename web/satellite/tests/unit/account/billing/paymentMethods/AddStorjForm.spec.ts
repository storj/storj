// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { mount } from '@vue/test-utils';

import AddStorjForm from '@/components/account/billing/paymentMethods/AddStorjForm.vue';

describe('AddStorjForm', () => {
    it('renders correctly', () => {
        const wrapper = mount<AddStorjForm>(AddStorjForm);

        expect(wrapper).toMatchSnapshot();
    });
});
