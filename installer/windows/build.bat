candle.exe -arch x64 StorageNode.wxs CustomInstallDir.wxs OperatorConfig.wxs

light.exe -o storagenode.msi -ext WixUIExtension StorageNode.wixobj CustomInstallDir.wixobj OperatorConfig.wixobj
