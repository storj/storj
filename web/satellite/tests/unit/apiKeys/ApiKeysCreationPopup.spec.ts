import { createLocalVue, mount } from '@vue/test-utils';
import Vuex from 'vuex';
import ApiKeysCreationPopup from '@/components/apiKeys/ApiKeysCreationPopup.vue';
import { ApiKey } from '@/types/apiKeys';
import { apiKeysModule } from '@/store/modules/apiKeys';

const localVue = createLocalVue();

localVue.use(Vuex);

let state = apiKeysModule.state;
let mutations = apiKeysModule.mutations;
let actions = apiKeysModule.actions;
let getters = apiKeysModule.getters;

const store = new Vuex.Store({
    modules: {
        apiKeysModule: {
            state,
            mutations,
            actions,
            getters
        }
    }
});

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

    // it('function onCopyClick works correctly', () => {
    //     const wrapper = mount(ApiKeysCreationPopup, {
    //         store,
    //         localVue,
    //     });
    //
    //     wrapper.vm.onCopyClick();
    //
    //     expect(wrapper.vm.$data.isCopiedButtonShown).toBe(true);
    // });

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

describe('ApiKeysArea async not success', () => {
    let store;
    let actions;
    let state;
    let getters;

    beforeEach(() => {
        actions = {
            createAPIKey: async () => {
                return {
                    errorMessage: '',
                    isSuccess: false,
                    data: null,
                };
            },
            error: jest.fn()
        };

        getters = {
            selectedAPIKeys: () => []
        };

        state = {
            apiKeys: []
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

    it('action on onNextClick while loading works correctly', async () => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        wrapper.vm.$data.isLoading = true;

        wrapper.vm.onNextClick();

        expect(wrapper.vm.$data.isLoading).toBe(true);
    });

    it('action on onNextClick works correctly', async () => {
        const wrapper = mount(ApiKeysCreationPopup, {
            store,
            localVue,
        });

        wrapper.vm.$data.isLoading = false;
        wrapper.vm.$data.name = 'testName';

        wrapper.vm.onNextClick();

        await actions.createAPIKey();

        expect(actions.error.mock.calls).toHaveLength(1);
        expect(wrapper.vm.$data.isLoading).toBe(false);
    });
});
