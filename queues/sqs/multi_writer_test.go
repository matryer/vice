package sqs

import (
	"strconv"
	"testing"
	"time"

	"github.com/matryer/vice/v2"
	"github.com/matryer/vice/v2/queues/sqs/sqsfakes"
	"github.com/matryer/vice/v2/vicetest"

	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiTransport(t *testing.T) {
	svc := &mockSQSClient{
		chs:    make(map[string]chan string),
		finish: make(chan bool),
	}

	newT := func() vice.Transport {
		transport := NewMulti(1, 0, 10*time.Second)
		transport.NewService = func(region string) (sqsiface.SQSAPI, error) {
			return svc, nil
		}

		return transport
	}

	vicetest.Transport(t, newT)
	close(svc.finish)
}

func Test_MultiTransport_SingleWriterBatchesWrites(t *testing.T) {
	newTransport := NewMulti(1, 10, 1000*time.Millisecond)
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
		assert.Contains(t, *batch0.Entries[i].MessageBody, strconv.Itoa(i))
	}

	batch1 := svc.SendMessageBatchArgsForCall(1)
	require.Equal(t, 10, len(batch1.Entries))
	for i := 0; i < 10; i++ {
		assert.Contains(t, *batch1.Entries[i].MessageBody, strconv.Itoa(i))
	}

	batch2 := svc.SendMessageBatchArgsForCall(2)
	require.Equal(t, 10, len(batch2.Entries))
	for i := 0; i < 10; i++ {
		assert.Contains(t, *batch2.Entries[i].MessageBody, strconv.Itoa(i))
	}
}

func Test_MultiTransport_SingleWriterBatchFlushesOnReturn(t *testing.T) {
	newTransport := NewMulti(1, 10, 1000*time.Millisecond)
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

func Test_MultiTransport_MultipleWritersBatchesWrites(t *testing.T) {
	newTransport := NewMulti(3, 10, 1000*time.Millisecond)
	var svcs []*sqsfakes.FakeSQSAPI
	for i := 0; i < 3; i++ {
		svc := new(sqsfakes.FakeSQSAPI)
		svc.SendMessageBatchStub = func(input *sqs.SendMessageBatchInput) (*sqs.SendMessageBatchOutput, error) {
			time.Sleep(200 * time.Millisecond)
			return &sqs.SendMessageBatchOutput{}, nil
		}
		svcs = append(svcs, svc)
	}

	var numWriters int
	newTransport.NewService = func(region string) (sqsiface.SQSAPI, error) {
		svc := svcs[numWriters]
		numWriters++
		return svc, nil
	}

	stream := newTransport.Send("svcWriter")
	for i := 0; i < 30; i++ {
		stream <- []byte(strconv.Itoa(i))
	}
	time.Sleep(100 * time.Millisecond)

	require.Equal(t, 3, numWriters)

	for i := 0; i < 3; i++ {
		svc := svcs[i]
		require.Equal(t, 1, svc.SendMessageBatchCallCount())
		batch := svc.SendMessageBatchArgsForCall(0)
		require.Equal(t, 10, len(batch.Entries))
		for j := 0; j < 10; j++ {
			assert.Contains(t, *batch.Entries[j].MessageBody, strconv.Itoa(j))
		}
	}
}
