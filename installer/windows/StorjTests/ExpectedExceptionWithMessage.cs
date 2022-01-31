using System;
using Microsoft.VisualStudio.TestTools.UnitTesting;

/// <summary>
/// Helper class that allows checking both the type and the message of the thrown exception.
/// </summary>
public class ExpectedExceptionWithMessageAttribute : ExpectedExceptionBaseAttribute
{
    public Type ExceptionType { get; set; }

    public string ExpectedMessage { get; set; }

    public ExpectedExceptionWithMessageAttribute(Type exceptionType)
    {
        this.ExceptionType = exceptionType;
    }

    public ExpectedExceptionWithMessageAttribute(Type exceptionType, string expectedMessage)
    {
        this.ExceptionType = exceptionType;
        this.ExpectedMessage = expectedMessage;
    }

    protected override void Verify(Exception e)
    {
        if (e.GetType() != this.ExceptionType)
        {
            throw new AssertFailedException(string.Format(
                    "Test method threw exception {0}, but exception {1} was expected. Exception message: {1}: {2}",
                    this.ExceptionType.FullName,
                    e.GetType().FullName,
                    e.Message
                ));
        }

        var actualMessage = e.Message.Trim();

        if (this.ExpectedMessage != null && !this.ExpectedMessage.Equals(actualMessage))
        {
            throw new AssertFailedException(string.Format(
                    "Test method threw exception with message '{0}', but message '{1}' was expected.",
                    actualMessage,
                    this.ExpectedMessage
                ));
        }
    }
}