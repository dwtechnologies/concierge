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
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"

	//	"github.com/aws/aws-sdk-go/aws/session"
	lambdasdk "github.com/aws/aws-sdk-go-v2/service/lambda"

	//	"github.com/aws/aws-sdk-go-v2/aws/session"

	//	"github.com/aws/aws-sdk-go/aws"
	//	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go-v2/service/iot"
	"github.com/aws/aws-sdk-go-v2/service/iotdataplane"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	//	"github.com/aws/aws-sdk-go/service/lambda"
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

	//LERRA HACK
	if os.Getenv("MFA_ARN") != "" {
		if *userID == "Lezgin" {
			mfa, err := mfaInvoke("lezgin.bakircioglu")
			if err != nil {
				return err
			}
			//fmt.Sprintf("%c", mfa)
			fmt.Printf("%s", mfa)
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

/*
	body, err := json.Marshal(map[string]interface{}{
		"Function": "MFA-AUTH",
		"Authuser": username,
	})
	type Payload struct {
		Body string `json:"body"`
	}
	p := Payload{
		Body: string(body),
	}
	payload, err := json.Marshal(p) // This should give you {"body":"{\"name\":\"Jimmy\"}"} if you print it out which is the required format for the lambda request body.
	//https://stackoverflow.com/questions/52153716/how-do-you-invoke-a-lambda-function-within-another-lambda-function-using-golang
	log.Printf("Requesting MFA request for username %s", username)
	region := os.Getenv("AWS_REGION")
	region = "eu-west-1"
	session, err := session.NewSession(&aws.Config{ // Use aws sdk to connect to dynamoDB
		Region: &region})
	if err != nil {
		log.Printf("%s", err)
		return err
	}
	return nil
}
*/
/*
//func mfaRequest(username) {
func mfaRequest() {
	session, err := session.NewSession(&aws.Config{ // Use aws sdk to connect to dynamoDB
		//Region: &region,
		Region: "eu-west-1",
	})
	svc := invoke.New(session)
	body, err := json.Marshal(map[string]interface{}{
		"Function": "MFA-AUTH",
		"Authuser": "lezgin.bakircioglu",
	})

	type Payload struct {
		// You can also include more objects in the structure like below,
		// but for my purposes body was all that was required
		// Method string `json:"httpMethod"`
		Body string `json:"body"`
	}
	p := Payload{
		// Method: "POST",
		Body: string(body),
	}
	payload, err := json.Marshal(p)
	// Result should be: {"body":"{\"name\":\"Jimmy\"}"}
	// This is the required format for the lambda request body.

	if err != nil {
		fmt.Println("Json Marshalling error")
	}
	fmt.Println(string(payload))

	input := &invoke.InvokeInput{
		FunctionName:   aws.String("reliability-mfa-auth"),
		InvocationType: aws.String("RequestResponse"),
		LogType:        aws.String("Tail"),
		Payload:        payload,
	}
	result, err := svc.Invoke(input)

	if err != nil {
		fmt.Println("error")
		fmt.Println(err.Error())
	}
	var m map[string]interface{}
	json.Unmarshal(result.Payload, &m)
	fmt.Println(m["body"])

	invokeReponse, err := json.Marshal(m["body"])
	resp := Response{
		StatusCode:      200,
		IsBase64Encoded: false,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(invokeReponse),
	}

}
*/

func mfaInvoke(username string) ([]byte, error) {
	//        arn := "arn:aws:lambda:eu-west-1:494090316628:function:reliability-mfa-auth"
	arn := os.Getenv("MFA_ARN")
	arnSplit := strings.Split(arn, ":")
	if len(arnSplit) != 7 {
		return nil, fmt.Errorf("ARN pattern is not correct. Got %s", arn)
	}

	region := arnSplit[3]
	function := arnSplit[6]
	//fmt.Printf("%s %s \n", region, function)
	/*
		body, err := json.Marshal(map[string]interface{}{
			"Function": "MFA-Auth",
			"AuthUser": username,
		})
			type Payload struct {
				// You can also include more objects in the structure like below,
				// but for my purposes body was all that was required
				// Method string `json:"httpMethod"`
				Body string `json:"body"`
			}
			p := Payload{
				// Method: "POST",
				Body: string(body),
			}
			payload, err := json.Marshal(p)
	*/
	//	payload, err := json.Marshal(string(body))
	//	payload, err := json.RawMessage(string(body))

	payload, err := json.Marshal(mfaRequest{
		Function: "MFA-Auth",
		AuthUser: username,
	})
	if err != nil {
		fmt.Println("Json Marshalling error")
	}
	fmt.Println(string(payload))

	/*	//byte invoke
		invoke, error2 := lambdaInvoke(region, function, payload)
		if error2 != nil {
			return nil, error2
		}
		return nil, invoke
	*/
	return lambdaInvoke(region, function, payload)
}

func lambdaInvoke(region string, function string, payload []byte) ([]byte, error) {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic("failed to load config, " + err.Error())
	}
	//region=string.Split(arn,":")
	//arn:aws:lambda:eu-west-1:494090316628:function:reliability-mfa-auth
	//cfg.Region="eu-west-1"
	cfg.Region = region
	client := lambdasdk.New(cfg)

	log.Printf("lambda call (region,function,payload): %s %s %s", region, function, payload)

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

/*
policy needed:
              -
                Effect: "Allow"
                Action:
                  - "lambda:invokeFunction"
                Resource:
                                    - theInvokedLambdaArn*/
