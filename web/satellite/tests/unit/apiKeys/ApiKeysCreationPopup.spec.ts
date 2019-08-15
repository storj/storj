import { createLocalVue, mount } from '@vue/test-utils';
import Vuex from 'vuex';
import { ApiKey } from '@/types/apiKeys';
import { makeApiKeysModule } from '@/store/modules/apiKeys';
import ApiKeysCreationPopup from '@/components/apiKeys/ApiKeysCreationPopup.vue';

const localVue = createLocalVue();
localVue.use(Vuex);
const apiKeysModule = makeApiKeysModule();
const store = new Vuex.Store(apiKeysModule);

describe('ApiKeysCreationPopup', () => {
    let value = 'testValue';

    it('renders correctly', () => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('function onCloseClick works correctly', () => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        wrapper.vm.onCloseClick();

        expect(wrapper.emitted()).toEqual({'closePopup': [[]]});
    });

    it('function onChangeName works correctly', () => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        wrapper.vm.onChangeName(value);

        wrapper.vm.$data.name = value.trim();
        expect(wrapper.vm.$data.name).toMatch('testValue');
        expect(wrapper.vm.$data.errorMessage).toMatch('');
    });

    it('action on onNextClick with no name works correctly', async () => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        wrapper.vm.$data.isLoading = false;
        wrapper.vm.$data.name = '';

        await wrapper.vm.onNextClick();

        expect(wrapper.vm.$data.errorMessage).toMatch('API Key name can`t be empty');
    });
});

describe('ApiKeysArea async success', () => {
    let store;
    let actions;
    let state;
    let getters;
    let apiKey = new ApiKey('testId', 'test', 'test', 'test');

    beforeEach(() => {
        actions = {
            createAPIKey: async () => {
                return {
                    errorMessage: '',
                    isSuccess: true,
                    data: apiKey,
                };
            },
            success: jest.fn()
        };

        getters = {
            selectedAPIKeys: () => [apiKey]
        };

        state = {
            apiKeys: [apiKey]
        };

        store = new Vuex.Store({
            modules: {
                apiKeysModule: {
                    state,
                    actions,
                    getters
                }
            }
        });
    });

    it('action on onNextClick with name works correctly', async () => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        wrapper.vm.$data.isLoading = false;
        wrapper.vm.$data.name = 'testName';

        wrapper.vm.onNextClick();

        let result = await actions.createAPIKey();

        expect(actions.success.mock.calls).toHaveLength(1);
        expect(wrapper.vm.$data.key).toBe(result.data.secret);
        expect(wrapper.vm.$data.isLoading).toBe(false);
        expect(wrapper.emitted()).toEqual({'closePopup': [[]], 'showCopyPopup': [['test']]});
    });
});

