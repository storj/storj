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
        [ExpectedException(typeof(ArgumentException), "The value cannot be empty.")]
        public void NullStorage()
        {
            CustomActionRunner.ValidateStorage(null, StorageDir);
        }

        [TestMethod]
        [ExpectedException(typeof(ArgumentException), "The value cannot be empty.")]
        public void EmptyStorage()
        {
            CustomActionRunner.ValidateStorage("", StorageDir);
        }

        [TestMethod]
        [ExpectedException(typeof(ArgumentException), "'some random text' is not a valid number.")]
        public void InvalidNumber()
        {
            CustomActionRunner.ValidateStorage("some random text", StorageDir);
        }

        [TestMethod]
        [ExpectedException(typeof(ArgumentException), "The allocated disk space cannot be less than 0.5 TB.")]
        public void TooSmall()
        {
            CustomActionRunner.ValidateStorage("0.41", StorageDir);
        }

        [TestMethod]
        [ExpectedException(typeof(ArgumentException), "The storage directory cannot be null")]
        public void NullStorageDir()
        {
            CustomActionRunner.ValidateStorage("3.14", null);
        }

        [TestMethod]
        [ExpectedException(typeof(ArgumentException), "The storage directory cannot be null")]
        public void EmptyStorageDir()
        {
            CustomActionRunner.ValidateStorage("3.14", "The path is not of a legal form.");
        }

        // TODO: add tests that mock the file system
        // [TestMethod]
        // public void ValidStorage()
        // {
        //     CustomActionRunner.ValidateStorage("3.14", StorageDir);
        // }
    }
}
