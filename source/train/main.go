package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type requestBody struct {
	Action string `json:"action"`
	Key    string `json:"key"`
	Name   string `json:"name"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	lambda.Start(handler)
}

func handler(event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("event: %s", event.Body)
	bucketName := os.Getenv("BUCKET_NAME")

	b := &requestBody{}
	json.Unmarshal([]byte(event.Body), b)

	// s3 client
	cfg, err := external.LoadDefaultAWSConfig()
	s3Client := s3.New(cfg)

	if b.Action == "train" {
		log.Printf("training model on s3 object: %s/%s", bucketName, b.Key)
		rekognitionClient := rekognition.New(cfg)
		_, err = rekognitionClient.IndexFacesRequest(&rekognition.IndexFacesInput{
			CollectionId: aws.String(os.Getenv("REKOGNITION_COLLECTION_ID")),
			Image: &rekognition.Image{
				S3Object: &rekognition.S3Object{
					Bucket: aws.String(bucketName),
					Name:   aws.String(b.Key),
				},
			},
			ExternalImageId: aws.String(b.Name),
			DetectionAttributes: []rekognition.Attribute{
				rekognition.AttributeDefault,
			},
		}).Send()
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}

		newKey := fmt.Sprintf("trained/%s/%s.jpg", b.Name, fmt.Sprintf("%x", md5.Sum([]byte(b.Key))))

		// copy s3 object to trained folder
		log.Printf("copying s3 object %s/%s to %s/%s", bucketName, b.Key, bucketName, newKey)
		_, err = s3Client.CopyObjectRequest(&s3.CopyObjectInput{
			CopySource: aws.String(bucketName + "/" + b.Key),
			Bucket:     aws.String(bucketName),
			Key:        aws.String(newKey),
			ACL:        s3.ObjectCannedACLPublicRead,
		}).Send()
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}
	}

	// remove original s3 object
	log.Printf("discarding s3 object: %s/%s", bucketName, b.Key)
	_, err = s3Client.DeleteObjectRequest(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(b.Key),
	}).Send()
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"CARD-ACTION-STATUS": "model " + b.Action + "ed successfully",
		},
	}, nil
}
