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
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type APIResponse struct {
	Message  string `json:"message"`
}

type Response events.APIGatewayProxyResponse

var cfg aws.Config
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
		pollyClient = polly.New(cfg)
	}

	input := &polly.SynthesizeSpeechInput{
		Text:         aws.String(message),
		TextType:     polly.TextTypeText,
		VoiceId:      polly.VoiceIdTakumi,
		LanguageCode: polly.LanguageCodeJaJp,
		OutputFormat: polly.OutputFormatMp3,
	}
	req := pollyClient.SynthesizeSpeechRequest(input)
	res, err := req.Send(ctx)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	io.Copy(buf, res.SynthesizeSpeechOutput.AudioStream)
	data := buf.Bytes()
	contentType := "audio/mp3"
	filename := t.Format(layout2) + ".mp3"
	uploader := s3manager.NewUploader(cfg)
	_, err = uploader.Upload(&s3manager.UploadInput{
		ACL: s3.ObjectCannedACLPublicRead,
		Bucket: aws.String(os.Getenv("BUCKET_NAME")),
		Key: aws.String(filename),
		Body: bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}
	url := "https://" + os.Getenv("BUCKET_NAME") + ".s3-" + os.Getenv("REGION") + ".amazonaws.com/" + filename
	return url, nil
}

func init() {
	var err error
	cfg, err = external.LoadDefaultAWSConfig()
	cfg.Region = os.Getenv("REGION")
	if err != nil {
		log.Print(err)
	}
}

func main() {
	lambda.Start(HandleRequest)
}
