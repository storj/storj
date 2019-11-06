using System;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using Storj;

namespace StorjTests
{
    [TestClass]
    public class ValidateBandwidthTests
    {
        [TestMethod]
        [ExpectedException(typeof(ArgumentException), "The value cannot be empty.")]
        public void NullBandwidth()
        {
            CustomActionRunner.ValidateBandwidth(null);
        }

        [TestMethod]
        [ExpectedException(typeof(ArgumentException), "The value cannot be empty.")]
        public void EmptyBandwidth()
        {
            CustomActionRunner.ValidateBandwidth("");
        }

        [TestMethod]
        [ExpectedException(typeof(ArgumentException), "'some random text' is not a valid number.")]
        public void InvalidNumber()
        {
            CustomActionRunner.ValidateBandwidth("some random text");
        }

        [TestMethod]
        [ExpectedException(typeof(ArgumentException), "The allocated bandwidth cannot be less than 2 TB.")]
        public void TooSmall()
        {
            CustomActionRunner.ValidateBandwidth("1.41");
        }

        [TestMethod]
        public void ValidBandwidth()
        {
            CustomActionRunner.ValidateBandwidth("3.14");
        }
    }
}
