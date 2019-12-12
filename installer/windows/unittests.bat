rem install NuGet packages
nuget install installer\windows\Storj\packages.config -o installer\windows\packages
nuget install installer\windows\StorjTests\packages.config -o installer\windows\packages

rem build the test project
msbuild installer\windows\StorjTests

rem run the unit tests
vstest.console installer\windows\StorjTests\bin\Debug\StorjTests.dll
