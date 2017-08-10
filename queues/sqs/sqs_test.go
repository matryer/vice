package sqs

import (
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/matryer/is"
	"github.com/matryer/vice"
	"github.com/matryer/vice/vicetest"
)

func TestTransport(t *testing.T) {
	svc := &mockSQSClient{
		chs:    make(map[string]chan string),
		finish: make(chan bool),
	}

	new := func() vice.Transport {
		transport := New()
		transport.NewService = func(region string) (sqsiface.SQSAPI, error) {
			return svc, nil
		}

		return transport
	}

	vicetest.Transport(t, new)
	close(svc.finish)
}

func TestParseRegion(t *testing.T) {
	is := is.New(t)
	reg := RegionFromURL("http://sqs.us-east-2.amazonaws.com/123456789012/MyQueue")
	is.Equal("us-east-2", reg)

	reg = RegionFromURL("http://localhost/foo")
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
	ch, ok := m.chs[q]
	if !ok {
		ch = make(chan string, 200)
		m.chs[q] = ch
	}
	m.mutex.Unlock()

	ch <- *s.MessageBody
	return &sqs.SendMessageOutput{}, nil
}

func (m *mockSQSClient) ReceiveMessage(s *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
	q := *s.QueueUrl
	m.mutex.Lock()
	ch, ok := m.chs[q]
	if !ok {
		ch = make(chan string, 200)
		m.chs[q] = ch
	}
	m.mutex.Unlock()

	out := &sqs.ReceiveMessageOutput{}
	msg := &sqs.Message{}

	select {
	case temp := <-ch:
		msg.Body = &temp
		out.Messages = append(out.Messages, msg)
	case <-m.finish:
	}
	return out, nil
}
