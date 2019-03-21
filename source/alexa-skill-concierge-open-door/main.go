package main

import (
	"context"
	"errors"
	"log"
	"encoding/json"
	"os"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iotdataplane"
	"github.com/aws/aws-sdk-go-v2/service/iot"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/ericdaugherty/alexa-skills-kit-golang"
)
// var a = &alexa.Alexa{ApplicationID: "amzn1.ask.skill.ccbf9fdc-c2da-49b2-9422-1d7434fef622", RequestHandler: &HelloWorld{}, IgnoreTimestamp: true}
var a = &alexa.Alexa{ApplicationID: os.Getenv("ALEXA_APPLICATION_ID"), RequestHandler: &HelloWorld{}, IgnoreTimestamp: true}

const cardTitle = "HelloWorld"

// HelloWorld handles reqeusts from the HelloWorld skill.
type HelloWorld struct{}

// Handle processes calls from Lambda
func Handle(ctx context.Context, requestEnv *alexa.RequestEnvelope) (interface{}, error) {
	return a.ProcessRequest(ctx, requestEnv)
}

// OnSessionStarted called when a new session is created.
func (h *HelloWorld) OnSessionStarted(context context.Context, request *alexa.Request, session *alexa.Session, aContext *alexa.Context, response *alexa.Response) error {

	log.Printf("OnSessionStarted requestId=%s, sessionId=%s", request.RequestID, session.SessionID)

	return nil
}

// OnLaunch called with a reqeust is received of type LaunchRequest
func (h *HelloWorld) OnLaunch(context context.Context, request *alexa.Request, session *alexa.Session, aContext *alexa.Context, response *alexa.Response) error {
	speechText := "Welcome to your personal concierge, you can ask doorman to open the door"

	log.Printf("OnLaunch requestId=%s, sessionId=%s", request.RequestID, session.SessionID)

	response.SetSimpleCard(cardTitle, speechText)
	response.SetOutputText(speechText)
	response.SetRepromptText(speechText)

	response.ShouldSessionEnd = false

	return nil
}

// OnIntent called with a reqeust is received of type IntentRequest
func (h *HelloWorld) OnIntent(context context.Context, request *alexa.Request, session *alexa.Session, aContext *alexa.Context, response *alexa.Response) error {

	log.Printf("OnIntent requestId=%s, sessionId=%s, intent=%s", request.RequestID, session.SessionID, request.Intent.Name)

	switch request.Intent.Name {
	case "open":
		log.Println("concierge triggered")
		speechText := "Sure, opening the door now"

		response.SetSimpleCard(cardTitle, speechText)
		response.SetOutputText(speechText)

		log.Printf("Set Output speech, value now: %s", response.OutputSpeech.Text)
		openDoor()
	case "AMAZON.HelpIntent":
		log.Println("AMAZON.HelpIntent triggered")
		speechText := "You can tell me, alexa ask doorman to open the door"

		response.SetSimpleCard("HelloWorld", speechText)
		response.SetOutputText(speechText)
		response.SetRepromptText(speechText)
	default:
		return errors.New("Invalid Intent")
	}

	return nil
}

// OnSessionEnded called with a reqeust is received of type SessionEndedRequest
func (h *HelloWorld) OnSessionEnded(context context.Context, request *alexa.Request, session *alexa.Session, aContext *alexa.Context, response *alexa.Response) error {

	log.Printf("OnSessionEnded requestId=%s, sessionId=%s", request.RequestID, session.SessionID)

	return nil
}

func main() {
	lambda.Start(Handle)
}

func openDoor() error {

	iotTopic := os.Getenv("IOT_TOPIC")
	iotTopic = "doorman"
        cfg, err := external.LoadDefaultAWSConfig()
        log.Printf("publishing to iot-data topic %s ", iotTopic)
        // get iot endpoint
        iotClient := iot.New(cfg)
        result, err := iotClient.DescribeEndpointRequest(&iot.DescribeEndpointInput{}).Send()
//        log.Printf("publishing to iot-endpoint %s ", *result.EndpointAddress)
        if err != nil {
                return err
        }
        cfg.EndpointResolver = aws.ResolveWithEndpointURL("https://" + *result.EndpointAddress)

        iotDataClient := iotdataplane.New(cfg)
        p := struct {
                Username string `json:"username"`
                Command  string `json:"command"`
        }{
//                "amzn1.ask.skill.ccbf9fdc-c2da-49b2-9422-1d7434fef622",
                os.Getenv("ALEXA_APPLICATION_ID"),
                "open",
        }
        pp, _ := json.Marshal(p)
        log.Printf("publishing the json %+v\n", pp)
        _, err = iotDataClient.PublishRequest(&iotdataplane.PublishInput{
                Payload: pp,
                Topic:   aws.String(iotTopic),
                Qos:     aws.Int64(0),
        }).Send()
        if err != nil {
                return err
        }
	return nil
}
