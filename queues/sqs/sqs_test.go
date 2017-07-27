package sqs

import (
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/cheekybits/is"
	"github.com/matryer/vice/test"
)

func TestTransport(t *testing.T) {
	transport := New()
	svc := &mockSQSClient{
		chs:    make(map[string]chan string),
		finish: make(chan bool),
	}

	transport.NewService = func(region string) sqsiface.SQSAPI {
		return svc
	}
	test.Transport(t, transport)
	close(svc.finish)
}

func TestParseRegion(t *testing.T) {
	is := is.New(t)
	reg := RegionFromUrl("http://sqs.us-east-2.amazonaws.com/123456789012/MyQueue")
	is.Equal("us-east-2", reg)

	reg = RegionFromUrl("http://localhost/foo")
	is.Equal("", reg)
}

type mockSQSClient struct {
	sqsiface.SQSAPI
	chs    map[string]chan string
	mutex  sync.Mutex
	finish chan bool
}

func (m *mockSQSClient) SendMessage(s *sqs.SendMessageInput) (*sqs.SendMessageOutput, error) {
	q := *s.QueueUrl
	m.mutex.Lock()
	if _, ok := m.chs[q]; !ok {
		m.chs[q] = make(chan string, 200)
	}
	m.mutex.Unlock()

	m.chs[q] <- *s.MessageBody
	return &sqs.SendMessageOutput{}, nil
}

func (m *mockSQSClient) ReceiveMessage(s *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
	q := *s.QueueUrl
	m.mutex.Lock()
	if _, ok := m.chs[q]; !ok {
		m.chs[q] = make(chan string, 200)
	}
	m.mutex.Unlock()

	out := &sqs.ReceiveMessageOutput{}
	msg := &sqs.Message{}

	select {
	case temp := <-m.chs[q]:
		msg.Body = &temp
		out.Messages = append(out.Messages, msg)
	case <-m.finish:
	}
	return out, nil
}
