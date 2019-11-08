using System;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using Storj;

namespace StorjTests
{
    [TestClass]
    public class ValidateStorageDirTests
    {
        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "You must select a storage folder.")]
        public void NullStorageDir()
        {
            CustomActionRunner.ValidateStorageDir(null);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "You must select a storage folder.")]
        public void EmptyStorageDir()
        {
            CustomActionRunner.ValidateStorageDir("");
        }

        // TODO: add tests that mock the file system
    }
}
