export declare function needToProduceProvide(original: any): boolean;
interface ProvideObj {
    managed?: {
        [k: string]: any;
    };
    managedReactive?: {
        [k: string]: any;
    };
}
declare type ProvideFunc = ((this: any) => Object) & ProvideObj;
export declare function produceProvide(original: any): ProvideFunc;
export {};
