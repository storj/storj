candle.exe -arch x64 Product.wxs CustomInstallDir.wxs OperatorConfig.wxs

light.exe -o storagenode.msi -ext WixUIExtension Product.wixobj CustomInstallDir.wixobj OperatorConfig.wixobj
