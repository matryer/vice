package sqs

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/matryer/vice/queues/sqs/sqsfakes"
	"github.com/matryer/vice/v2"
	"github.com/matryer/vice/vicetest"

	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/matryer/is"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransport(t *testing.T) {
	svc := &mockSQSClient{
		chs:    make(map[string]chan string),
		finish: make(chan bool),
	}

	new := func() vice.Transport {
		transport := New(0, 10*time.Second)
		transport.NewService = func(region string) (sqsiface.SQSAPI, error) {
			return svc, nil
		}

		return transport
	}

	vicetest.Transport(t, new)
	close(svc.finish)
}

func Test_Transport_BatchesWrites(t *testing.T) {
	newTransport := New(10, 200*time.Millisecond)
	svc := new(sqsfakes.FakeSQSAPI)
	svc.SendMessageBatchReturns(&sqs.SendMessageBatchOutput{}, nil)
	newTransport.NewService = func(region string) (sqsiface.SQSAPI, error) { return svc, nil }

	stream := newTransport.Send("svcWriter")
	for i := 0; i < 30; i++ {
		stream <- []byte(strconv.Itoa(i))
	}
	time.Sleep(100 * time.Millisecond)

	require.Equal(t, 3, svc.SendMessageBatchCallCount())

	batch0 := svc.SendMessageBatchArgsForCall(0)
	require.Equal(t, 10, len(batch0.Entries))
	for i := 0; i < 10; i++ {
		assert.Equal(t, strconv.Itoa(i), *batch0.Entries[i].MessageBody)
	}

	batch1 := svc.SendMessageBatchArgsForCall(1)
	require.Equal(t, 10, len(batch1.Entries))
	for i := 0; i < 10; i++ {
		assert.Equal(t, strconv.Itoa(i+10), *batch1.Entries[i].MessageBody)
	}

	batch2 := svc.SendMessageBatchArgsForCall(2)
	require.Equal(t, 10, len(batch2.Entries))
	for i := 0; i < 10; i++ {
		assert.Equal(t, strconv.Itoa(i+20), *batch2.Entries[i].MessageBody)
	}
}

func Test_Transport_BatchFlushesOnReturn(t *testing.T) {
	newTransport := New(10, 1000*time.Millisecond)
	svc := new(sqsfakes.FakeSQSAPI)
	svc.SendMessageBatchReturns(&sqs.SendMessageBatchOutput{}, nil)
	newTransport.NewService = func(region string) (sqsiface.SQSAPI, error) { return svc, nil }

	stream := newTransport.Send("svcWriter")
	for i := 0; i < 9; i++ {
		stream <- []byte(strconv.Itoa(i))
	}
	time.Sleep(100 * time.Millisecond)
	newTransport.Stop()

	time.Sleep(200 * time.Millisecond)

	require.Equal(t, 1, svc.SendMessageBatchCallCount())

	batch0 := svc.SendMessageBatchArgsForCall(0)
	require.Equal(t, 9, len(batch0.Entries))
	for i := 0; i < 9; i++ {
		assert.Equal(t, strconv.Itoa(i), *batch0.Entries[i].MessageBody)
	}
}

func TestParseRegion(t *testing.T) {
	is := is.New(t)
	reg := RegionFromURL("http://sqs.us-east-2.amazonaws.com/123456789012/MyQueue")
	is.Equal("us-east-2", reg)

	reg = RegionFromURL("http://localhost/svcWriter")
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
