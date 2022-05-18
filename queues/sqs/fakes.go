//go:build fakes

package sqsfakes

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 github.com/aws/aws-sdk-go/service/sqs/sqsiface.SQSAPI

import _ "github.com/maxbrunsfeld/counterfeiter/v6"
