package main

import (
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"rdsauditlogss3/internal/database"
	"rdsauditlogss3/internal/logcollector"
	"rdsauditlogss3/internal/parser"
	"rdsauditlogss3/internal/processor"
	"rdsauditlogss3/internal/s3writer"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"strings"
)

// HandlerConfig holds the configuration for the lambda function
type HandlerConfig struct {
	RdsInstanceIdentifier string `envconfig:"RDS_INSTANCE_IDENTIFIER" required:"true" desc:"Identifier of the RDS instance"`
	S3BucketName          string `envconfig:"S3_BUCKET_NAME" required:"true" desc:"Name of the bucket to write logs to"`
	DynamoDbTableName     string `envconfig:"DYNAMODB_TABLE_NAME" required:"true" desc:"DynamoDb table name"`
	AwsRegion             string `envconfig:"AWS_REGION" required:"true" desc:"AWS region"`
	Debug                 bool   `envconfig:"DEBUG" required:"true" desc:"Enable debug mode."`
	RDSInterfaceEndpoint string `envconfig:"RDS_INTERFACE_ENDPOINT" required:"false" desc:"RDS interface endpoint"`
}

type lambdaHandler struct {
	processor *processor.Processor
}

// Handler is the handler registered as the lambda function handler
func (lh *lambdaHandler) Handler() error {
	err := lh.processor.Process()
	if err != nil {
		if strings.Contains(err.Error(), "file was rotated when getting the logs") {
			log.Warn(err.Error())
			return nil
		}
		log.WithError(err).Errorf("Error in Lambda function")
		return fmt.Errorf("error in Lambda function")
	}
	return nil
}

func main() {
	// use JSONFormatter
	log.SetFormatter(&log.JSONFormatter{})
	var c HandlerConfig
	err := envconfig.Process("", &c)
	if err != nil {
		log.WithError(err).Fatal("Error parsing configuration")
	}

	if c.Debug {
		log.SetLevel(log.DebugLevel)
	}

    // Initialize AWS session with custom endpoint resolver
    sessionConfig := &aws.Config{
        Region: aws.String(c.AwsRegion),
        // Define custom endpoint resolver
        EndpointResolver: endpoints.ResolverFunc(func(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
            if service == "rds" && c.RDSInterfaceEndpoint != "" {
                // Replace "your-custom-endpoint" with your actual custom RDS endpoint
                return endpoints.ResolvedEndpoint{
                    URL: "https://" + c.RDSInterfaceEndpoint,
                }, nil
            }
            // Return default resolver for other services
            return endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
        }),
    }


	// Initialize AWS session
	// sessionConfig := &aws.Config{
	// 	Region: aws.String(c.AwsRegion),
	// }
	sess := session.New(sessionConfig)

	// Create & start lambda handler
	lh := &lambdaHandler{
		processor: processor.NewProcessor(
			database.NewDynamoDb(
				dynamodb.New(sess),
				c.DynamoDbTableName,
			),
			logcollector.NewRdsLogCollector(
				rds.New(sess),
				logcollector.NewAWSHttpClient(sess),
				c.AwsRegion,
				c.RdsInstanceIdentifier,
				"mysql",
				c.RDSInterfaceEndpoint,
			),
			s3writer.NewS3Writer(
				s3manager.NewUploader(sess),
				c.S3BucketName,
				fmt.Sprintf("%s/%s", c.RdsInstanceIdentifier, "audit-logs"),
			),
			parser.NewAuditLogParser(),
			c.RdsInstanceIdentifier,
		),
	}
	lambda.Start(lh.Handler)
}
