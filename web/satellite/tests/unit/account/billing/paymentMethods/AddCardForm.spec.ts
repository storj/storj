// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import AddCardForm from '@/components/account/billing/paymentMethods/AddCardForm.vue';

import { NotificatorPlugin } from '@/utils/plugins/notificator';
import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();
const notificationsPlugin = new NotificatorPlugin();
localVue.use(notificationsPlugin);

describe('AddCardForm', () => {
    it('renders correctly', () => {
        const wrapper = mount(AddCardForm, {
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
