package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go-v2/service/iot"
	"github.com/aws/aws-sdk-go-v2/service/iotdataplane"
	lambdasdk "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/disintegration/imaging"
)

type teamsResponse struct {
	Type       string          `json:"@type"`
	Context    string          `json:"@context"`
	ThemeColor string          `json:"themeColor"`
	Summary    string          `json:"summary"`
	Title      string          `json:"title"`
	Text       string          `json:"text"`
	Sections   json.RawMessage `json:"sections"`
}

type mfaRequest struct {
	Function string `json:"Function"`
	AuthUser string `json:"AuthUser"`
}

/*
type mfaResponse struct {
        Function   string          `json:"function"`
        Authuser   string          `json:"authuser"`
}
*/
const smallHeight = 400

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	lambda.Start(handler)
}

func handler(event events.S3Event) error {
	key := event.Records[0].S3.Object.Key
	bucketName := os.Getenv("BUCKET_NAME")
	iotTopic := os.Getenv("IOT_TOPIC")
	teamsWebhookURL := os.Getenv("TEAMS_WEBHOOK")

	cfg, err := external.LoadDefaultAWSConfig()

	// SearchFacesByImageRequest to rekognition
	rekClient := rekognition.New(cfg)
	rekResp, err := rekClient.SearchFacesByImageRequest(&rekognition.SearchFacesByImageInput{
		CollectionId: aws.String(os.Getenv("REKOGNITION_COLLECTION_ID")),
		Image: &rekognition.Image{
			S3Object: &rekognition.S3Object{
				Bucket: aws.String(bucketName),
				Name:   aws.String(key),
			},
		},
		MaxFaces:           aws.Int64(1),
		FaceMatchThreshold: aws.Float64(70),
	}).Send()
	if err != nil {
		return err
	}

	s3Client := s3.New(cfg)
	if len(rekResp.FaceMatches) == 0 {
		log.Printf("no matches found, sending to unknown folder")

		newKey := fmt.Sprintf("unknown/%s.jpg", fmt.Sprintf("%x", md5.Sum([]byte(key))))

		log.Printf("copying s3 object %s/%s to %s/%s", bucketName, key, bucketName, newKey)
		_, err = s3Client.CopyObjectRequest(&s3.CopyObjectInput{
			CopySource: aws.String(bucketName + "/" + key),
			Bucket:     aws.String(bucketName),
			Key:        aws.String(newKey),
			ACL:        s3.ObjectCannedACLPublicRead,
		}).Send()
		if err != nil {
			return err
		}

		log.Printf("discarding s3 object: %s/%s", bucketName, key)
		_, err = s3Client.DeleteObjectRequest(&s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		}).Send()
		if err != nil {
			return err
		}

		return nil
	}

	log.Printf("face found")
	log.Printf("SearchFacesByImageRequest response: %s", rekResp)

	userID := rekResp.FaceMatches[0].Face.ExternalImageId
	newKey := fmt.Sprintf("detected/%s/%s.jpg", *userID, fmt.Sprintf("%x", md5.Sum([]byte(key))))

	log.Printf("copying s3 object %s/%s to %s/%s", bucketName, key, bucketName, newKey)
	_, err = s3Client.CopyObjectRequest(&s3.CopyObjectInput{
		CopySource: aws.String(bucketName + "/" + key),
		Bucket:     aws.String(bucketName),
		Key:        aws.String(newKey),
		ACL:        s3.ObjectCannedACLPublicRead,
	}).Send()
	if err != nil {
		return err
	}

	log.Printf("discarding s3 object: %s/%s", bucketName, key)
	_, err = s3Client.DeleteObjectRequest(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}).Send()
	if err != nil {
		return err
	}

	// rate limit
	if os.Getenv("OPEN_RATE_SECONDS") != "" {
		grace, _ := strconv.ParseInt(os.Getenv("OPEN_RATE_SECONDS"), 10, 64)

		if rateLimit(grace, *userID) {
			log.Printf("[%s] rate limit triggered for open event", *userID)
			return nil
		}
	}

	// MFA support via external Lambda middleware function
	if os.Getenv("MFA_ARN") != "" {
		if *userID == "Lezgin" {
			mfa, err := mfaInvoke("lezgin.bakircioglu")
			if err != nil {
				return err
			}
			log.Printf("mfa: %s", mfa)
			//if return not auth then exit
		}
	}

	log.Printf("publishing to iot-data topic %s ", iotTopic)
	// get iot endpoint
	iotClient := iot.New(cfg)
	result, err := iotClient.DescribeEndpointRequest(&iot.DescribeEndpointInput{}).Send()
	if err != nil {
		return err
	}
	cfg.EndpointResolver = aws.ResolveWithEndpointURL("https://" + *result.EndpointAddress)

	iotDataClient := iotdataplane.New(cfg)
	p := struct {
		Username string `json:"username"`
		Command  string `json:"command"`
	}{
		*userID,
		"open",
	}

	pp, _ := json.Marshal(p)
	_, err = iotDataClient.PublishRequest(&iotdataplane.PublishInput{
		Payload: pp,
		Topic:   aws.String(iotTopic),
		Qos:     aws.Int64(0),
	}).Send()
	if err != nil {
		return err
	}

	// thumbnail
	keySmall := newKey + "_small"
	err = thumbnail(bucketName, newKey)
	if err != nil {
		log.Printf("error resizing %s", err)
		keySmall = newKey
	}

	log.Printf("sending welcome message to Teams")
	body, _ := json.Marshal(teamsResponse{
		Type:       "MessageCard",
		Context:    "http://schema.org/extensions",
		ThemeColor: "ccc",
		Title:      fmt.Sprintf("welcome to the office %s", *userID),
		Text:       fmt.Sprintf("![who](https://s3.amazonaws.com/%s/%s)", bucketName, keySmall),
	})

	req, err := http.NewRequest("POST", teamsWebhookURL, bytes.NewBuffer(body))
	req.Header.Add("Content-Type", "application/json")
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("teams - error: %v", err)

		return fmt.Errorf("teams - error")
	}
	defer resp.Body.Close()

	b, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("teams - response code: %v", resp.StatusCode)
		log.Printf("teams - body: %s", b) // debug

		return fmt.Errorf("teams - unreachable")
	}

	return nil
}

func rateLimit(grace int64, userID string) bool {
	now := time.Now().Unix()
	cutoff := now - grace

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return false
	}
	client := dynamodb.New(cfg)

	av, err := dynamodbattribute.MarshalMap(struct {
		Name      string `json:"name"`
		Selector  string `json:"selector"`
		Timestamp int64  `json:"timestamp"`
	}{
		Name:      "open",
		Selector:  userID,
		Timestamp: now,
	})
	if err != nil {
		return false
	}

	_, err = client.PutItemRequest(&dynamodb.PutItemInput{
		TableName:           aws.String(os.Getenv("DYNAMODB_TABLE_RATELIMIT")),
		Item:                av,
		ConditionExpression: aws.String(fmt.Sprintf("attribute_not_exists(name) | attribute(timestamp) < %d", cutoff)),
	}).Send()
	if err != nil {
		return false
	}

	return true
}

func thumbnail(bucketName, key string) error {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return err
	}
	client := s3.New(cfg)

	log.Printf("s3 GET object %s/%s", bucketName, key)
	result, err := client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}).Send()
	if err != nil {
		log.Printf("%s", err)
		return err
	}

	log.Printf("decoding image")
	srcimg, err := imaging.Decode(result.Body)
	if err != nil {
		log.Printf("%s", err)
		return err
	}

	log.Printf("resizing image")
	dstimg := imaging.Resize(srcimg, 0, smallHeight, imaging.Linear)

	buf := new(bytes.Buffer)
	// err = imaging.Encode(buf, dstimg, imaging.JPEG)
	imaging.Encode(buf, dstimg, imaging.JPEG, imaging.JPEGQuality(90))
	if err != nil {
		log.Printf("%s", err)
		return err
	}

	log.Printf("s3 PUT object %s/%s_resized", bucketName, key)
	_, err = client.PutObjectRequest(&s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key + "_small"),
		Body:        bytes.NewReader(buf.Bytes()),
		ACL:         s3.ObjectCannedACLPublicRead,
		ContentType: aws.String("image/jpeg"),
	}).Send()
	if err != nil {
		log.Printf("%s", err)
		return err
	}

	return nil
}

func mfaInvoke(username string) ([]byte, error) {
	arn := os.Getenv("MFA_ARN")
	arnSplit := strings.Split(arn, ":")
	if len(arnSplit) != 7 {
		return nil, fmt.Errorf("ARN pattern is not correct. Got %s", arn)
	}

	region := arnSplit[3]
	function := arnSplit[6]
	payload, err := json.Marshal(mfaRequest{
		Function: "MFA-Auth",
		AuthUser: username,
	})
	if err != nil {
		fmt.Println("Json Marshalling error")
	}
	log.Println(string(payload))

	// invoke MFA lambda
	return lambdaInvoke(region, function, payload)
}

func lambdaInvoke(region, function string, payload []byte) ([]byte, error) {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return nil, err
	}
	cfg.Region = region
	client := lambdasdk.New(cfg)

	log.Printf("lambda call (region: %s, function: %s,payload: %s)", region, function, payload)

	res, err := client.InvokeRequest(&lambdasdk.InvokeInput{
		FunctionName: aws.String(function),
		Payload:      payload,
	}).Send()
	if err != nil {
		log.Printf("%v", err)
		return nil, err
	}

	log.Printf("%s response: %v", function, res)
	return res.Payload, nil
}
