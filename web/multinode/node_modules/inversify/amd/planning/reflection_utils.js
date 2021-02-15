var __spreadArrays = (this && this.__spreadArrays) || function () {
    for (var s = 0, i = 0, il = arguments.length; i < il; i++) s += arguments[i].length;
    for (var r = Array(s), k = 0, i = 0; i < il; i++)
        for (var a = arguments[i], j = 0, jl = a.length; j < jl; j++, k++)
            r[k] = a[j];
    return r;
};
define(["require", "exports", "../annotation/inject", "../constants/error_msgs", "../constants/literal_types", "../constants/metadata_keys", "../utils/serialization", "./target"], function (require, exports, inject_1, ERROR_MSGS, literal_types_1, METADATA_KEY, serialization_1, target_1) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.getFunctionName = exports.getBaseClassDependencyCount = exports.getDependencies = void 0;
    Object.defineProperty(exports, "getFunctionName", { enumerable: true, get: function () { return serialization_1.getFunctionName; } });
    function getDependencies(metadataReader, func) {
        var constructorName = serialization_1.getFunctionName(func);
        var targets = getTargets(metadataReader, constructorName, func, false);
        return targets;
    }
    exports.getDependencies = getDependencies;
    function getTargets(metadataReader, constructorName, func, isBaseClass) {
        var metadata = metadataReader.getConstructorMetadata(func);
        var serviceIdentifiers = metadata.compilerGeneratedMetadata;
        if (serviceIdentifiers === undefined) {
            var msg = ERROR_MSGS.MISSING_INJECTABLE_ANNOTATION + " " + constructorName + ".";
            throw new Error(msg);
        }
        var constructorArgsMetadata = metadata.userGeneratedMetadata;
        var keys = Object.keys(constructorArgsMetadata);
        var hasUserDeclaredUnknownInjections = (func.length === 0 && keys.length > 0);
        var iterations = (hasUserDeclaredUnknownInjections) ? keys.length : func.length;
        var constructorTargets = getConstructorArgsAsTargets(isBaseClass, constructorName, serviceIdentifiers, constructorArgsMetadata, iterations);
        var propertyTargets = getClassPropsAsTargets(metadataReader, func);
        var targets = __spreadArrays(constructorTargets, propertyTargets);
        return targets;
    }
    function getConstructorArgsAsTarget(index, isBaseClass, constructorName, serviceIdentifiers, constructorArgsMetadata) {
        var targetMetadata = constructorArgsMetadata[index.toString()] || [];
        var metadata = formatTargetMetadata(targetMetadata);
        var isManaged = metadata.unmanaged !== true;
        var serviceIdentifier = serviceIdentifiers[index];
        var injectIdentifier = (metadata.inject || metadata.multiInject);
        serviceIdentifier = (injectIdentifier) ? (injectIdentifier) : serviceIdentifier;
        if (serviceIdentifier instanceof inject_1.LazyServiceIdentifer) {
            serviceIdentifier = serviceIdentifier.unwrap();
        }
        if (isManaged) {
            var isObject = serviceIdentifier === Object;
            var isFunction = serviceIdentifier === Function;
            var isUndefined = serviceIdentifier === undefined;
            var isUnknownType = (isObject || isFunction || isUndefined);
            if (!isBaseClass && isUnknownType) {
                var msg = ERROR_MSGS.MISSING_INJECT_ANNOTATION + " argument " + index + " in class " + constructorName + ".";
                throw new Error(msg);
            }
            var target = new target_1.Target(literal_types_1.TargetTypeEnum.ConstructorArgument, metadata.targetName, serviceIdentifier);
            target.metadata = targetMetadata;
            return target;
        }
        return null;
    }
    function getConstructorArgsAsTargets(isBaseClass, constructorName, serviceIdentifiers, constructorArgsMetadata, iterations) {
        var targets = [];
        for (var i = 0; i < iterations; i++) {
            var index = i;
            var target = getConstructorArgsAsTarget(index, isBaseClass, constructorName, serviceIdentifiers, constructorArgsMetadata);
            if (target !== null) {
                targets.push(target);
            }
        }
        return targets;
    }
    function getClassPropsAsTargets(metadataReader, constructorFunc) {
        var classPropsMetadata = metadataReader.getPropertiesMetadata(constructorFunc);
        var targets = [];
        var keys = Object.keys(classPropsMetadata);
        for (var _i = 0, keys_1 = keys; _i < keys_1.length; _i++) {
            var key = keys_1[_i];
            var targetMetadata = classPropsMetadata[key];
            var metadata = formatTargetMetadata(classPropsMetadata[key]);
            var targetName = metadata.targetName || key;
            var serviceIdentifier = (metadata.inject || metadata.multiInject);
            var target = new target_1.Target(literal_types_1.TargetTypeEnum.ClassProperty, targetName, serviceIdentifier);
            target.metadata = targetMetadata;
            targets.push(target);
        }
        var baseConstructor = Object.getPrototypeOf(constructorFunc.prototype).constructor;
        if (baseConstructor !== Object) {
            var baseTargets = getClassPropsAsTargets(metadataReader, baseConstructor);
            targets = __spreadArrays(targets, baseTargets);
        }
        return targets;
    }
    function getBaseClassDependencyCount(metadataReader, func) {
        var baseConstructor = Object.getPrototypeOf(func.prototype).constructor;
        if (baseConstructor !== Object) {
            var baseConstructorName = serialization_1.getFunctionName(baseConstructor);
            var targets = getTargets(metadataReader, baseConstructorName, baseConstructor, true);
            var metadata = targets.map(function (t) {
                return t.metadata.filter(function (m) {
                    return m.key === METADATA_KEY.UNMANAGED_TAG;
                });
            });
            var unmanagedCount = [].concat.apply([], metadata).length;
            var dependencyCount = targets.length - unmanagedCount;
            if (dependencyCount > 0) {
                return dependencyCount;
            }
            else {
                return getBaseClassDependencyCount(metadataReader, baseConstructor);
            }
        }
        else {
            return 0;
        }
    }
    exports.getBaseClassDependencyCount = getBaseClassDependencyCount;
    function formatTargetMetadata(targetMetadata) {
        var targetMetadataMap = {};
        targetMetadata.forEach(function (m) {
            targetMetadataMap[m.key.toString()] = m.value;
        });
        return {
            inject: targetMetadataMap[METADATA_KEY.INJECT_TAG],
            multiInject: targetMetadataMap[METADATA_KEY.MULTI_INJECT_TAG],
            targetName: targetMetadataMap[METADATA_KEY.NAME_TAG],
            unmanaged: targetMetadataMap[METADATA_KEY.UNMANAGED_TAG]
        };
    }
});
