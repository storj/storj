using System;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using Storj;

namespace StorjTests
{
    [TestClass]
    public class ValidateBandwidthTests
    {
        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The value cannot be empty.")]
        public void NullBandwidth()
        {
            new CustomActionRunner().ValidateBandwidth(null);
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The value cannot be empty.")]
        public void EmptyBandwidth()
        {
            new CustomActionRunner().ValidateBandwidth("");
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "'some random text' is not a valid number.")]
        public void InvalidNumber()
        {
            new CustomActionRunner().ValidateBandwidth("some random text");
        }

        [TestMethod]
        [ExpectedExceptionWithMessage(typeof(ArgumentException), "The allocated bandwidth cannot be less than 2 TB.")]
        public void TooSmall()
        {
            new CustomActionRunner().ValidateBandwidth("1.41");
        }

        [TestMethod]
        public void ValidBandwidth()
        {
            new CustomActionRunner().ValidateBandwidth("3.14");
        }
    }
}
