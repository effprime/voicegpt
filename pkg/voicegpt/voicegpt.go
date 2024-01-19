package voicegpt

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"

	speech "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/speech/apiv1/speechpb"
	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"github.com/effprime/voicegpt/pkg/gptclient"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	OpenAIKeyEnvKey           = "OPENAI_KEY"
	GoogleCloudCredentialsKey = "GOOGLE_APPLICATION_CREDENTIALS" // this is read by Google's lib, can't change
)

type VoiceGPTHandler struct {
	openAIKey string
	opts      *VoiceGPTOptions
	sessions  *FileSessionStorage
}

type VoiceGPTOptions struct {
	GPTModel   string
	SessionDir string
}

func NewVoiceGPTHandler(opts *VoiceGPTOptions) (*VoiceGPTHandler, error) {
	openAIKey := os.Getenv(OpenAIKeyEnvKey)
	if openAIKey == "" {
		return nil, fmt.Errorf("OpenAI key not founded at env var: %s", OpenAIKeyEnvKey)
	}
	googleCredsPath := os.Getenv(GoogleCloudCredentialsKey)
	if googleCredsPath == "" {
		return nil, fmt.Errorf("Google Cloud credentials path not found at env var: %s", googleCredsPath)
	}
	sessions, err := NewFileSessionStorage(opts.SessionDir)
	if err != nil {
		return nil, err
	}
	return &VoiceGPTHandler{
		openAIKey: openAIKey,
		opts:      opts,
		sessions:  sessions,
	}, nil
}

type Request struct {
	SessionID string
	VoiceData io.ReadSeeker
}

type Response struct {
	SessionID   string
	Transcript  string
	GPTResponse string
	VoiceData   io.Reader
}

func (v *VoiceGPTHandler) Handle(ctx context.Context, req *Request) (*Response, error) {
	voiceData, err := io.ReadAll(req.VoiceData)
	if err != nil {
		return nil, err
	}

	log.Printf("Received %v bytes of voice data", len(voiceData))

	transcript, err := transcribeSpeech(ctx, voiceData)
	if err != nil {
		return nil, err
	}

	log.Printf("Transcript: %s", transcript)

	session, err := v.sessions.Get(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}

	messages := []gptclient.Message{
		{
			Role:    gptclient.RoleSystem,
			Content: "You are responding to a text message that was transcribed from audio. It likely will miss punctuation especially periods. Do your best to make sense of it. The text that you return will be synthesized back to speech, so please do not return extremely long responses.",
		},
	}
	newMessage := gptclient.Message{
		Role:    gptclient.RoleUser,
		Content: transcript,
	}
	if session != nil {
		messages = append(messages, session.Messages...)
		session.Messages = append(session.Messages, newMessage)
	} else {
		session = &Session{ID: uuid.New().String(), Messages: []gptclient.Message{newMessage}}
	}
	messages = append(messages, newMessage)

	gptClient := gptclient.NewClient(v.openAIKey)
	gptResp, err := gptClient.Chat(&gptclient.ChatCompletionRequest{
		Model:    v.opts.GPTModel,
		Messages: messages,
	})
	if err != nil {
		return nil, err
	}
	if len(gptResp.Choices) == 0 {
		return nil, fmt.Errorf("received empty ChatGPT response (no choices)")
	}

	session.Messages = append(session.Messages, gptResp.Choices[0].Message)
	err = v.sessions.Save(ctx, session)
	if err != nil {
		return nil, err
	}

	textToSpeech, err := synthesizeText(ctx, gptResp.Choices[0].Message.Content)
	if err != nil {
		return nil, err
	}

	return &Response{
		SessionID:   session.ID,
		Transcript:  transcript,
		GPTResponse: gptResp.Choices[0].Message.Content,
		VoiceData:   bytes.NewBuffer(textToSpeech),
	}, nil
}

func transcribeSpeech(ctx context.Context, data []byte) (string, error) {
	c, err := speech.NewClient(ctx)
	if err != nil {
		return "", err
	}
	defer c.Close()

	speechReq := &speechpb.LongRunningRecognizeRequest{
		Config: &speechpb.RecognitionConfig{
			Encoding:                            speechpb.RecognitionConfig_WEBM_OPUS,
			SampleRateHertz:                     48000,
			AudioChannelCount:                   1,
			EnableSeparateRecognitionPerChannel: false,
			LanguageCode:                        "en-US",
			AlternativeLanguageCodes:            []string{},
			MaxAlternatives:                     0,
			ProfanityFilter:                     false,
			Adaptation:                          &speechpb.SpeechAdaptation{},
			TranscriptNormalization:             &speechpb.TranscriptNormalization{},
			SpeechContexts:                      []*speechpb.SpeechContext{},
			EnableWordTimeOffsets:               false,
			EnableWordConfidence:                false,
			EnableAutomaticPunctuation:          false,
			EnableSpokenPunctuation:             &wrapperspb.BoolValue{},
			EnableSpokenEmojis:                  &wrapperspb.BoolValue{},
			DiarizationConfig:                   &speechpb.SpeakerDiarizationConfig{},
			Metadata:                            &speechpb.RecognitionMetadata{},
			Model:                               "",
			UseEnhanced:                         false,
		},
		Audio: &speechpb.RecognitionAudio{
			AudioSource: &speechpb.RecognitionAudio_Content{
				Content: data,
			},
		},
		OutputConfig: &speechpb.TranscriptOutputConfig{},
	}

	op, err := c.LongRunningRecognize(ctx, speechReq)
	if err != nil {
		return "", nil
	}

	speechResp, err := op.Wait(ctx)
	if err != nil {
		return "", nil
	}

	transcript := ""
	for _, result := range speechResp.Results {
		for _, alt := range result.Alternatives {
			transcript += alt.Transcript + " "
		}
	}
	return transcript, nil
}

func synthesizeText(ctx context.Context, text string) ([]byte, error) {
	client, err := texttospeech.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	req := &texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: "en-US",
			SsmlGender:   texttospeechpb.SsmlVoiceGender_NEUTRAL,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding: texttospeechpb.AudioEncoding_MP3,
		},
	}

	resp, err := client.SynthesizeSpeech(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.AudioContent, nil
}
