// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';
import { VNode } from 'vue';
import { DirectiveBinding } from 'vue/types/options';

import HistoryDropdown from '@/components/account/billing/HistoryDropdown.vue';

import { RouteConfig } from '@/router';
import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();

let clickOutsideEvent: EventListener;

localVue.directive('click-outside', {
    bind: function (el: HTMLElement, binding: DirectiveBinding, vnode: VNode) {
        clickOutsideEvent = function(event: Event): void {
            if (el === event.target) {
                return;
            }

            if (vnode.context) {
                vnode.context[binding.expression](event);
            }
        };

        document.body.addEventListener('click', clickOutsideEvent);
    },
    unbind: function(): void {
        document.body.removeEventListener('click', clickOutsideEvent);
    },
});

describe('HistoryDropdown', (): void => {
    it('renders correctly if credit history', (): void => {
        const creditsHistory: string = RouteConfig.Account.with(RouteConfig.CreditsHistory).path;
        const wrapper = mount(HistoryDropdown, {
            localVue,
            propsData: {
                label: 'Credits History',
                link: creditsHistory,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if balance history', (): void => {
        const balanceHistory: string = RouteConfig.Account.with(RouteConfig.DepositHistory).path;
        const wrapper = mount(HistoryDropdown, {
            localVue,
            propsData: {
                label: 'Balance History',
                link: balanceHistory,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('clicks work correctly', async (): Promise<void> => {
        const clickSpy = sinon.spy();
        const wrapper = mount(HistoryDropdown, {
            localVue,
            methods: {
                redirect: clickSpy,
            },
        });

        await wrapper.find('.history-dropdown__link-container').trigger('click');

        expect(clickSpy.callCount).toBe(1);
    });
});
