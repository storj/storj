using System;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using Storj;

namespace StorjTests
{
    [TestClass]
    public class ValidateStorageTests
    {
        private const string StorageDir = "X:\\storage";

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The value cannot be empty.")]
        public void NullStorage()
        {
            CustomActionRunner.ValidateStorage(null, StorageDir);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The value cannot be empty.")]
        public void EmptyStorage()
        {
            CustomActionRunner.ValidateStorage("", StorageDir);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "'some random text' is not a valid number.")]
        public void InvalidNumber()
        {
            CustomActionRunner.ValidateStorage("some random text", StorageDir);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The allocated disk space cannot be less than 0.5 TB.")]
        public void TooSmall()
        {
            CustomActionRunner.ValidateStorage("0.41", StorageDir);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "10000000 TB is too large value for allocated storage.")]
        public void TooLarge()
        {
            CustomActionRunner.ValidateStorage("10000000", StorageDir);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The storage directory cannot be empty")]
        public void NullStorageDir()
        {
            CustomActionRunner.ValidateStorage("3.14", null);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The storage directory cannot be empty")]
        public void EmptyStorageDir()
        {
            CustomActionRunner.ValidateStorage("3.14", "");
        }

        // TODO: add tests that mock the file system
        // [TestMethod]
        // public void ValidStorage()
        // {
        //     CustomActionRunner.ValidateStorage("3.14", StorageDir);
        // }
    }
}
