package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
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

const smallHeight = 400

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	lambda.Start(handler)
}

func handler(event events.S3Event) error {
	key := event.Records[0].S3.Object.Key
	bucketName := os.Getenv("BUCKET_NAME")
	teamsWebhookURL := os.Getenv("TEAMS_WEBHOOK")
	trainURL := os.Getenv("TRAIN_URL")

	// thumbnail
	keySmall := key + "_small"
	err := thumbnail(bucketName, key)
	if err != nil {
		log.Printf("error resizing %s", err)
		keySmall = key
	}

	body, _ := json.Marshal(teamsResponse{
		Type:       "MessageCard",
		Context:    "http://schema.org/extensions",
		ThemeColor: "ccc",
		Summary:    "I don't know who this is...",
		Title:      "I don't know who this is...",
		Text:       fmt.Sprintf("![who](https://s3.amazonaws.com/%s/%s)", bucketName, keySmall),
		Sections: json.RawMessage(fmt.Sprintf(`[{
			"potentialAction": [{
				"@type": "ActionCard",
				"name": "who",
				"inputs": [{
					"@type": "TextInput",
					"id": "name",
					"placeholder": "name",
					"title": "whodisis"
				}],
				"actions": [{
					"@type": "HttpPOST",
					"name": "Submit",
					"target": "%s",
					"body": "{\"action\": \"train\", \"key\": \"%s\", \"name\": \"{{name.value}}\"}",
					"headers": [{
						"Content-Type": "application/json"
				    	}]
			    	},{
					"@type": "HttpPOST",
					"name": "Discard",
					"target": "%s",
					"body": "{\"action\": \"discard\", \"key\": \"%s\"}",
					"headers": [{
						"Content-Type": "application/json"
				    	}]
			    	}]	
			}]
		}]`, trainURL, key, trainURL, key)),
	})

	req, err := http.NewRequest("POST", teamsWebhookURL, bytes.NewBuffer(body))
	req.Header.Add("Content-Type", "application/json")
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	log.Printf("posting teams message to %s", teamsWebhookURL)
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

	return err
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
