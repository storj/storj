package put

import (
	"github.com/spf13/cobra"
	"github.com/minio/minio-go/pkg/s3utils"
	"fmt"
	"errors"
)

//TODO: Add bucket name check
func validateArgs(cmd *cobra.Command, args []string) (error) {
	length := len(args)
	if length != 2 {
		return NewInvalidArgsError(length)
	}

	err := s3utils.CheckValidBucketName(args[0])
	if err != nil {
		return errors.New(fmt.Sprintf("%s. \n Please refer to this bucket naming conventions " +
			"(https://docs.aws.amazon.com/AmazonS3/latest/dev/BucketRestrictions.html", err.Error()))
	}

	return nil
}