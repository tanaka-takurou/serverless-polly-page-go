package main

import (
	"io"
	"fmt"
	"log"
	"time"
	"bytes"
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/polly"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type APIResponse struct {
	Message  string `json:"message"`
}

type Response events.APIGatewayProxyResponse

const layout       string = "2006-01-02 15:04"
const layout2      string = "20060102150405"
const languageCode string = "ja-JP"
const outputFormat string = "mp3"
const bucketName   string = "your-bucket-name"
const bucketRegion string = "ap-northeast-1"

func HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	var jsonBytes []byte
	var err error
	d := make(map[string]string)
	json.Unmarshal([]byte(request.Body), &d)
	if v, ok := d["action"]; ok {
		switch v {
		case "synthesizespeech" :
			if m, ok := d["message"]; ok {
				url, e := synthesizeSpeech(m)
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

func synthesizeSpeech(message string)(string, error) {
	t := time.Now()
	svc := polly.New(session.New(), &aws.Config{
		Region: aws.String("ap-northeast-1"),
	})

	voiceId := "Takumi"
	input := &polly.SynthesizeSpeechInput{
		Text:         aws.String(message),
		TextType:     aws.String("text"),
		VoiceId:      aws.String(voiceId),
		LanguageCode: aws.String(languageCode),
		OutputFormat: aws.String(outputFormat),
	}
	res, err := svc.SynthesizeSpeech(input)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	io.Copy(buf, res.AudioStream)
	data := buf.Bytes()
	contentType := "audio/mp3"
	filename := t.Format(layout2) + ".mp3"
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String(bucketRegion)},
	)
	uploader := s3manager.NewUploader(sess)
	_, err = uploader.Upload(&s3manager.UploadInput{
		ACL: aws.String("public-read"),
		Bucket: aws.String(bucketName),
		Key: aws.String(filename),
		Body: bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		log.Print(err)
		return "", err
	}
	url := "https://" + bucketName + ".s3-" + bucketRegion + ".amazonaws.com/" + filename
	return url, nil
}

func main() {
	lambda.Start(HandleRequest)
}
