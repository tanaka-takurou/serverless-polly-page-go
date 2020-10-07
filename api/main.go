package main

import (
	"io"
	"os"
	"fmt"
	"log"
	"time"
	"bytes"
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	ptypes "github.com/aws/aws-sdk-go-v2/service/polly/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	stypes "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type APIResponse struct {
	Message  string `json:"message"`
}

type Response events.APIGatewayProxyResponse

var s3Client *s3.Client
var pollyClient *polly.Client

const layout  string = "2006-01-02 15:04"
const layout2 string = "20060102150405"

func HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	var jsonBytes []byte
	var err error
	d := make(map[string]string)
	json.Unmarshal([]byte(request.Body), &d)
	if v, ok := d["action"]; ok {
		switch v {
		case "synthesizespeech" :
			if m, ok := d["message"]; ok {
				url, e := synthesizeSpeech(ctx, m)
				if e != nil {
					err = e
				} else {
					jsonBytes, _ = json.Marshal(APIResponse{Message: url})
				}
			}
		}
	}
	log.Print(request.RequestContext.Identity.SourceIP)
	if err != nil {
		log.Print(err)
		jsonBytes, _ = json.Marshal(APIResponse{Message: fmt.Sprint(err)})
		return Response{
			StatusCode: 500,
			Body: string(jsonBytes),
		}, nil
	}
	return Response {
		StatusCode: 200,
		Body: string(jsonBytes),
	}, nil
}

func synthesizeSpeech(ctx context.Context, message string)(string, error) {
	t := time.Now()
	if pollyClient == nil {
		pollyClient = getPollyClient()
	}

	input := &polly.SynthesizeSpeechInput{
		Text:         aws.String(message),
		TextType:     ptypes.TextTypeText,
		VoiceId:      ptypes.VoiceIdTakumi,
		LanguageCode: ptypes.LanguageCodeJaJp,
		OutputFormat: ptypes.OutputFormatMp3,
	}
	res, err := pollyClient.SynthesizeSpeech(ctx, input)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	io.Copy(buf, res.AudioStream)
	data := buf.Bytes()
	contentType := "audio/mp3"
	filename := t.Format(layout2) + ".mp3"
	if s3Client == nil {
		s3Client = getS3Client()
	}
	input_ := &s3.PutObjectInput{
		ACL: stypes.ObjectCannedACLPublicRead,
		Bucket: aws.String(os.Getenv("BUCKET_NAME")),
		Key: aws.String(filename),
		Body: bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}
	_, err = s3Client.PutObject(ctx, input_)
	if err != nil {
		return "", err
	}
	url := "https://" + os.Getenv("BUCKET_NAME") + ".s3-" + os.Getenv("REGION") + ".amazonaws.com/" + filename
	return url, nil
}

func getPollyClient() *polly.Client {
	return polly.NewFromConfig(getConfig())
}

func getS3Client() *s3.Client {
	return s3.NewFromConfig(getConfig())
}

func getConfig() aws.Config {
	cfg, err := config.LoadDefaultConfig()
	if err != nil {
		log.Print(err)
	}
	cfg.Region = os.Getenv("REGION")
	return cfg
}

func main() {
	lambda.Start(HandleRequest)
}
