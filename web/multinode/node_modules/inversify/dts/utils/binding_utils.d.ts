import { interfaces } from "../inversify";
export declare const multiBindToService: (container: interfaces.Container) => (service: interfaces.ServiceIdentifier<any>) => (...types: interfaces.ServiceIdentifier<any>[]) => void;
